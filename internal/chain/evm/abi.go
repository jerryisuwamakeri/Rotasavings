package evm

// Minimal hand-written ABIs for the methods and events the TruthLayer uses.
// They mirror contracts/src/*.sol. Generating full bindings with abigen after
// `forge build` is the production path; these keep the Go layer self-contained.

const identityRegistryABI = `[
  {"type":"function","name":"register","stateMutability":"nonpayable",
   "inputs":[{"name":"commitment","type":"bytes32"}],"outputs":[]},
  {"type":"function","name":"isRegistered","stateMutability":"view",
   "inputs":[{"name":"commitment","type":"bytes32"}],"outputs":[{"name":"","type":"bool"}]}
]`

const reputationLedgerABI = `[
  {"type":"function","name":"record","stateMutability":"nonpayable",
   "inputs":[
     {"name":"user","type":"address"},
     {"name":"cycleIndex","type":"uint256"},
     {"name":"eventType","type":"uint8"},
     {"name":"amount","type":"uint256"}],"outputs":[]},
  {"type":"event","name":"ReputationEvent","anonymous":false,
   "inputs":[
     {"name":"user","type":"address","indexed":true},
     {"name":"group","type":"address","indexed":true},
     {"name":"cycleIndex","type":"uint256","indexed":false},
     {"name":"eventType","type":"uint8","indexed":false},
     {"name":"amount","type":"uint256","indexed":false},
     {"name":"timestamp","type":"uint64","indexed":false}]}
]`

const rotasavingsGroupABI = `[
  {"type":"function","name":"activate","stateMutability":"nonpayable",
   "inputs":[{"name":"_payoutOrder","type":"address[]"}],"outputs":[]},
  {"type":"function","name":"contribute","stateMutability":"nonpayable",
   "inputs":[{"name":"cycle","type":"uint256"},{"name":"expected","type":"bytes32"}],"outputs":[]},
  {"type":"function","name":"settle","stateMutability":"nonpayable",
   "inputs":[{"name":"cycle","type":"uint256"}],"outputs":[]}
]`

// reputationEventTypes maps the on-chain enum ordinals to domain event types.
// Order must match ReputationLedger.EventType in the Solidity source.
var reputationEventOrdinal = []string{
	"ContributionMade",
	"ContributionMissed",
	"PayoutReceived",
	"GroupExit",
	"GroupExpulsion",
}
