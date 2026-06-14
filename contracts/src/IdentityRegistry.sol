// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/// @title IdentityRegistry
/// @notice On-chain anchor for KYC identity commitments. Stores only a hash
///         H(user_id + KYC_provider_signature + timestamp) — never PII. A
///         commitment can be anchored exactly once, guaranteeing global
///         uniqueness across all groups while preserving privacy.
contract IdentityRegistry {
    /// @dev commitment => the address that anchored it.
    mapping(bytes32 => address) public owner;
    /// @dev commitment => unix time it was anchored.
    mapping(bytes32 => uint64) public anchoredAt;

    event IdentityRegistered(bytes32 indexed commitment, address indexed owner, uint64 timestamp);

    error AlreadyRegistered(bytes32 commitment);

    /// @notice Anchor a new identity commitment. Reverts if already present.
    function register(bytes32 commitment) external {
        if (owner[commitment] != address(0)) revert AlreadyRegistered(commitment);
        owner[commitment] = msg.sender;
        anchoredAt[commitment] = uint64(block.timestamp);
        emit IdentityRegistered(commitment, msg.sender, uint64(block.timestamp));
    }

    /// @notice True if a commitment has been anchored.
    function isRegistered(bytes32 commitment) external view returns (bool) {
        return owner[commitment] != address(0);
    }
}
