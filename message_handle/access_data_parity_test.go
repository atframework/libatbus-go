package libatbus_message_handle

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/atframework/libatbus-go/protocol"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// A change of curve, token, or command bytes must change the signed input or signature.
func TestAccessDataHandshakeAndCommandSignatures(t *testing.T) {
	const busID = 42
	const prefix = "123:1-2:42:"
	tokens := [][]byte{[]byte("first token"), []byte("second token")}
	check := func(ad *protocol.AccessData, plain string, keys [][]byte) {
		t.Helper()
		require.Equal(t, int64(123), ad.GetTimestamp())
		require.Equal(t, uint64(1), ad.GetNonce1())
		require.Equal(t, uint64(2), ad.GetNonce2())
		require.Len(t, ad.GetSignature(), len(keys))
		for i, key := range keys {
			mac := hmac.New(sha256.New, key)
			mac.Write([]byte(plain))
			require.Equal(t, mac.Sum(nil), ad.GetSignature()[i])
		}
	}
	for _, curve := range []protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE{
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP256R1,
		protocol.ATBUS_CRYPTO_KEY_EXCHANGE_TYPE_ATBUS_CRYPTO_KEY_EXCHANGE_SECP384R1,
	} {
		hello := &protocol.CryptoHandshakeData{Type: curve, PublicKey: []byte("public key")}
		ad := &protocol.AccessData{}
		GenerateAccessDataWithTimestamp(ad, busID, 1, 2, tokens, hello, 123)
		want := fmt.Sprintf("%s%d:%x", prefix, curve, sha256.Sum256(hello.PublicKey))
		require.Equal(t, want, MakeAccessDataPlaintextFromHandshake(busID, ad, hello))
		check(ad, want, tokens)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		joined string
	}{
		{"empty", nil, ""}, {"single", []string{"one"}, "one"}, {"multiple", []string{"one", "two"}, "onetwo"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &protocol.CustomCommandData{}
			for _, arg := range tc.args {
				cmd.Commands = append(cmd.Commands, &protocol.CustomCommandArgv{Arg: []byte(arg)})
			}
			for _, keys := range [][][]byte{tokens[:1], tokens} {
				ad, again := &protocol.AccessData{}, &protocol.AccessData{}
				GenerateAccessDataForCustomCommandWithTimestamp(ad, busID, 1, 2, keys, cmd, 123)
				GenerateAccessDataForCustomCommandWithTimestamp(again, busID, 1, 2, keys, cmd, 123)
				require.True(t, proto.Equal(ad, again))
				plain := MakeAccessDataPlaintextFromCustomCommand(busID, ad, cmd)
				require.Equal(t, fmt.Sprintf("%s%x", prefix, sha256.Sum256([]byte(tc.joined))), plain)
				check(ad, plain, keys)
			}
		})
	}
	empty := CalculateAccessDataSignature(nil, tokens[0], "")
	mac := hmac.New(sha256.New, tokens[0])
	require.Equal(t, mac.Sum(nil), empty)
	require.NotEqual(t, empty, CalculateAccessDataSignature(nil, tokens[0], "a"))
	require.NotEqual(t, empty, CalculateAccessDataSignature(nil, tokens[1], ""))
}
