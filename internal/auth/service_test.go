package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	plain := "messi10"
	hash := "$2a$10$rkj2vPdQGL4bUedU/2ynHu8rkJxM5CDhDcvXFj82emRs/57JnUMJy"
	_ = hash

	new_hash, err := HashPassword(plain)
	_ = new_hash
	require.NoError(t, err)

	require.True(t, ComparePasswords(new_hash, plain))

}
