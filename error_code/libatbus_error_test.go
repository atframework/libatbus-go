package libatbus_types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibatbusStrerrorKnownCodes(t *testing.T) {
	// Arrange
	cases := []struct {
		code ErrorType
		want string
	}{
		{EN_ATBUS_ERR_SUCCESS, "EN_ATBUS_ERR_SUCCESS(0): success"},
		{EN_ATBUS_ERR_PARAMS, "EN_ATBUS_ERR_PARAMS(-1): ATBUS parameter error"},
		{EN_ATBUS_ERR_CLOSING, "EN_ATBUS_ERR_CLOSING(-15): closing"},
		{EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT, "EN_ATBUS_ERR_ALGORITHM_NOT_SUPPORT(-16): algorithm not supported"},
		{EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET, "EN_ATBUS_ERR_MESSAGE_NOT_FINISH_YET(-17): message not finished yet"},
		{EN_ATBUS_ERR_ATNODE_NOT_FOUND, "EN_ATBUS_ERR_ATNODE_NOT_FOUND(-65): target node not found"},
		{EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT, "EN_ATBUS_ERR_CHANNEL_NOT_SUPPORT(-105): channel not supported"},
		{EN_ATBUS_ERR_SHM_MAP_FAILED, "EN_ATBUS_ERR_SHM_MAP_FAILED(-305): shared memory map failed"},
		{EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED, "EN_ATBUS_ERR_PIPE_LOCK_PATH_FAILED(-507): pipe lock path failed"},
		{EN_ATBUS_ERR_NOT_READY, "EN_ATBUS_ERR_NOT_READY(-607): not ready"},
	}

	// Act + Assert
	for _, tc := range cases {
		assert.Equal(t, tc.want, LibatbusStrerror(tc.code))
		assert.Equal(t, tc.want, tc.code.String())
		assert.Equal(t, tc.want, tc.code.Error())
	}
}

func TestLibatbusStrerrorUnknownCode(t *testing.T) {
	// Arrange
	unknown := ErrorType(-123456)

	// Act
	got := LibatbusStrerror(unknown)

	// Assert
	assert.Equal(t, "ATBUS_ERROR_TYPE(-123456): unknown", got)
}
