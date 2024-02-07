package govindexer

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGovABI(t *testing.T) {
	gsABI, err := getGovernorSettingsABI()
	require.NoError(t, err)

	testTopics := makeGovTopics()

	require.Equal(t, 3, len(testTopics))

	evDelaySet := gsABI.Events["VotingDelaySet"]
	require.Equal(t, evDelaySet.ID, testTopics[0][0])

	evPeriodSet := gsABI.Events["VotingPeriodSet"]
	require.Equal(t, evPeriodSet.ID, testTopics[1][0])

	evThresholdSet := gsABI.Events["ProposalThresholdSet"]
	require.Equal(t, evThresholdSet.ID, testTopics[2][0])

}
