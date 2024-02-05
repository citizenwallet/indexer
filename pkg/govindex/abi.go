package govindex

import (
	"encoding/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"strings"
)

const (
	// GovVotingDelaySet emitted at gov contract creation
	GovVotingDelaySet       = "VotingDelaySet(uint256,uint256)"
	GovVotingPeriodSet      = "VotingPeriodSet(uint256,uint256)"
	GovProposalThresholdSet = "ProposalThresholdSet(uint256,uint256)"
)

var (
	GovVotingDelaySetId       = crypto.Keccak256Hash([]byte(GovVotingDelaySet))
	GovVotingPeriodSetId      = crypto.Keccak256Hash([]byte(GovVotingPeriodSet))
	GovProposalThresholdSetId = crypto.Keccak256Hash([]byte(GovProposalThresholdSet))
)

func makeGovTopics() (topics [][]common.Hash) {
	topics = [][]common.Hash{
		{GovVotingDelaySetId, GovVotingPeriodSetId, GovProposalThresholdSetId},
	}
	return
}

func extractContractABI(jsonFile string) (*abi.ABI, error) {
	contractBytes, err := abiGovSettings.ReadFile(jsonFile)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err = json.Unmarshal(contractBytes, &m); err != nil {
		return nil, err
	}

	ma := m["abi"]
	abiBytes, err := json.Marshal(ma)
	if err != nil {
		return nil, err
	}

	abi, err := abi.JSON(strings.NewReader(string(abiBytes)))
	if err != nil {
		return nil, err
	}
	return &abi, nil
}
func getGovernorSettingsABI() (*abi.ABI, error) {
	return extractContractABI("abi/GovernorSettings.json")
}
