// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import {ReputationLedger} from "./ReputationLedger.sol";

/// @title RotasavingsGroup
/// @notice One deployed ROSCA group. Holds the immutable rules (contribution
///         amount, cycle length, payout order) and enforces the lifecycle
///         CREATED -> ACTIVE -> SETTLEMENT -> CLOSED. Contributions are bound to
///         a commitment H(user + group + cycle + amount); a default is the
///         deterministic failure to reveal it before the cycle deadline.
///
/// @dev The off-chain orchestration layer mirrors this state; the chain is the
///      source of truth. Real fund custody lives off-chain (escrow + payment
///      rails); this contract records truth and reputation, not money movement.
contract RotasavingsGroup {
    enum State {
        CREATED,
        ACTIVE,
        SETTLEMENT,
        CLOSED
    }

    address public immutable organizer;
    ReputationLedger public immutable ledger;

    uint256 public immutable contributionAmount;
    uint256 public immutable cycleLength; // seconds
    State public state;
    uint64 public activatedAt;

    address[] public members;
    address[] public payoutOrder; // one payee per cycle
    mapping(address => bool) public isMember;

    /// @dev cycleIndex => member => revealed contribution commitment.
    mapping(uint256 => mapping(address => bytes32)) public contributions;
    /// @dev cycleIndex => settled flag.
    mapping(uint256 => bool) public settled;

    event GroupActivated(uint64 timestamp, uint256 totalCycles);
    event ContributionRevealed(address indexed user, uint256 indexed cycle, bytes32 commitment);
    event DefaultRecorded(address indexed user, uint256 indexed cycle, uint256 missedAmount, bytes32 proof);
    event CyclePaidOut(address indexed payee, uint256 indexed cycle, uint256 amount);
    event StateChanged(State from, State to);

    error NotOrganizer();
    error BadState(State have, State want);
    error NotMember(address who);
    error CycleSettled(uint256 cycle);
    error BadCommitment();
    error AlreadyContributed(address who, uint256 cycle);

    modifier onlyOrganizer() {
        if (msg.sender != organizer) revert NotOrganizer();
        _;
    }

    modifier inState(State want) {
        if (state != want) revert BadState(state, want);
        _;
    }

    constructor(
        address _organizer,
        ReputationLedger _ledger,
        uint256 _contributionAmount,
        uint256 _cycleLength,
        address[] memory _members
    ) {
        organizer = _organizer;
        ledger = _ledger;
        contributionAmount = _contributionAmount;
        cycleLength = _cycleLength;
        members = _members;
        for (uint256 i = 0; i < _members.length; i++) {
            isMember[_members[i]] = true;
        }
        state = State.CREATED;
    }

    /// @notice Activate the group, fixing the payout order (one payee per cycle).
    function activate(address[] calldata _payoutOrder) external onlyOrganizer inState(State.CREATED) {
        require(_payoutOrder.length == members.length, "payout order must cover all members");
        payoutOrder = _payoutOrder;
        activatedAt = uint64(block.timestamp);
        _setState(State.ACTIVE);
        emit GroupActivated(activatedAt, _payoutOrder.length);
    }

    /// @notice Reveal a contribution commitment for a cycle.
    /// @param expected The commitment the caller claims; must match the on-chain
    ///        recomputation H(user, group, cycle, amount).
    function contribute(uint256 cycle, bytes32 expected) external inState(State.ACTIVE) {
        if (!isMember[msg.sender]) revert NotMember(msg.sender);
        if (settled[cycle]) revert CycleSettled(cycle);
        if (contributions[cycle][msg.sender] != bytes32(0)) revert AlreadyContributed(msg.sender, cycle);

        bytes32 commitment = keccak256(abi.encodePacked(msg.sender, address(this), cycle, contributionAmount));
        if (commitment != expected) revert BadCommitment();

        contributions[cycle][msg.sender] = commitment;
        ledger.record(msg.sender, cycle, ReputationLedger.EventType.ContributionMade, contributionAmount);
        emit ContributionRevealed(msg.sender, cycle, commitment);
    }

    /// @notice Cycle deadline (one cycleLength per index past activation).
    function cycleDeadline(uint256 cycle) public view returns (uint64) {
        return activatedAt + uint64((cycle + 1) * cycleLength);
    }

    /// @notice Settle a cycle whose deadline has passed: record defaults for any
    ///         member who did not contribute, then mark settled and pay out.
    function settle(uint256 cycle) external onlyOrganizer inState(State.ACTIVE) {
        if (settled[cycle]) revert CycleSettled(cycle);
        require(block.timestamp >= cycleDeadline(cycle), "cycle not yet due");

        uint256 collected;
        for (uint256 i = 0; i < members.length; i++) {
            address m = members[i];
            if (contributions[cycle][m] == bytes32(0)) {
                bytes32 proof = keccak256(abi.encodePacked(m, address(this), cycle, "default"));
                ledger.record(m, cycle, ReputationLedger.EventType.ContributionMissed, contributionAmount);
                emit DefaultRecorded(m, cycle, contributionAmount, proof);
            } else {
                collected += contributionAmount;
            }
        }

        settled[cycle] = true;
        address payee = payoutOrder[cycle];
        ledger.record(payee, cycle, ReputationLedger.EventType.PayoutReceived, collected);
        emit CyclePaidOut(payee, cycle, collected);

        if (cycle + 1 == payoutOrder.length) {
            _setState(State.SETTLEMENT);
            _setState(State.CLOSED);
        }
    }

    function memberCount() external view returns (uint256) {
        return members.length;
    }

    function _setState(State to) internal {
        emit StateChanged(state, to);
        state = to;
    }
}
