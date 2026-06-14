// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title ReputationLedger
/// @notice Append-only, event-based reputation. Reputation is NOT a score: it
///         is the deterministic aggregation of these immutable events, portable
///         across every group. Only authorised group contracts may emit.
contract ReputationLedger {
    enum EventType {
        ContributionMade,
        ContributionMissed,
        PayoutReceived,
        GroupExit,
        GroupExpulsion
    }

    address public immutable admin;
    /// @dev group contract address => allowed to write.
    mapping(address => bool) public authorized;

    event ReputationEvent(
        address indexed user,
        address indexed group,
        uint256 cycleIndex,
        EventType eventType,
        uint256 amount,
        uint64 timestamp
    );
    event WriterAuthorized(address indexed group, bool allowed);

    error NotAdmin();
    error NotAuthorized();

    constructor() {
        admin = msg.sender;
    }

    /// @notice Admin authorises (or revokes) a group contract to record events.
    function setAuthorized(address group, bool allowed) external {
        if (msg.sender != admin) revert NotAdmin();
        authorized[group] = allowed;
        emit WriterAuthorized(group, allowed);
    }

    /// @notice Record a reputation event. Callable only by an authorised group.
    function record(
        address user,
        uint256 cycleIndex,
        EventType eventType,
        uint256 amount
    ) external {
        if (!authorized[msg.sender]) revert NotAuthorized();
        emit ReputationEvent(user, msg.sender, cycleIndex, eventType, amount, uint64(block.timestamp));
    }
}
