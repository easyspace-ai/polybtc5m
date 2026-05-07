package polymarket

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ── L1 Auth (for API key creation / derivation) ─────────────────────────────

var (
	clobAuthTypeHash = crypto.Keccak256Hash([]byte(
		"ClobAuth(address address,string timestamp,uint256 nonce,string message)"))
	clobAuthDomainTypeHash = crypto.Keccak256Hash([]byte(
		"EIP712Domain(string name,string version,uint256 chainId)"))
)

// SignClobAuth creates an EIP-712 signature for CLOB L1 authentication.
// This is used when creating or deriving API keys on Polymarket CLOB V2.
func SignClobAuth(pk *ecdsa.PrivateKey, chainID int, timestamp int64, nonce int64) (string, error) {
	if pk == nil {
		return "", fmt.Errorf("private key is required")
	}

	domainSep := crypto.Keccak256Hash(
		clobAuthDomainTypeHash.Bytes(),
		hashString("ClobAuthDomain").Bytes(),
		hashString("1").Bytes(),
		common.LeftPadBytes(new(big.Int).SetInt64(int64(chainID)).Bytes(), 32),
	)

	timestampStr := strconv.FormatInt(timestamp, 10)
	message := "This message attests that I control the given wallet"
	addr := crypto.PubkeyToAddress(pk.PublicKey)

	structHash := crypto.Keccak256Hash(
		clobAuthTypeHash.Bytes(),
		common.LeftPadBytes(addr.Bytes(), 32),
		hashString(timestampStr).Bytes(),
		common.LeftPadBytes(new(big.Int).SetInt64(nonce).Bytes(), 32),
		hashString(message).Bytes(),
	)

	digest := crypto.Keccak256Hash([]byte{0x19, 0x01}, domainSep.Bytes(), structHash.Bytes())
	sig, err := crypto.Sign(digest.Bytes(), pk)
	if err != nil {
		return "", err
	}
	if sig[64] < 27 {
		sig[64] += 27
	}
	return "0x" + common.Bytes2Hex(sig), nil
}

// CreateL1Headers generates L1 authentication headers for API key operations
// (POST /auth/api-key or GET /auth/derive-api-key).
func CreateL1Headers(pk *ecdsa.PrivateKey, chainID int, nonce int64) (map[string]string, error) {
	now := time.Now()
	sig, err := SignClobAuth(pk, chainID, now.Unix(), nonce)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"POLY_ADDRESS":   strings.ToLower(crypto.PubkeyToAddress(pk.PublicKey).Hex()),
		"POLY_SIGNATURE": sig,
		"POLY_TIMESTAMP": strconv.FormatInt(now.Unix(), 10),
		"POLY_NONCE":     strconv.FormatInt(nonce, 10),
	}, nil
}

// ── Wallet Derivation (CREATE2) ─────────────────────────────────────────────

const (
	polygonProxyFactory = "0xaB45c5A4B0c941a2F231C04C3f49182e1A254052"
	polygonSafeFactory  = "0xaacFeEa03eb1561C4e67d661e40682Bd20E3541b"

	proxyInitCodeHash = "0xd21df8dc65880a8606f09fe0ce3df9b8869287ab0b058be05aa9e8af6330a00b"
	safeInitCodeHash  = "0x2bce2127ff07fb632d16c8347c4ebf501f4841168bed00d9e6ef715ddb6fcecf"
)

// DeriveSafeWallet derives the Gnosis Safe wallet address for an EOA using CREATE2.
// Salt = keccak256(address padded to 32 bytes, left-padded with zeros).
func DeriveSafeWallet(eoaAddress common.Address, chainID int) (common.Address, error) {
	factory, err := safeFactoryForChain(chainID)
	if err != nil {
		return common.Address{}, err
	}
	var padded [32]byte
	copy(padded[12:], eoaAddress.Bytes())
	salt := crypto.Keccak256Hash(padded[:])
	return create2(factory, salt, common.HexToHash(safeInitCodeHash)), nil
}

// DeriveProxyWallet derives the Polymarket Proxy wallet address for an EOA using CREATE2.
// Salt = keccak256(raw 20-byte address).
func DeriveProxyWallet(eoaAddress common.Address, chainID int) (common.Address, error) {
	factory, err := proxyFactoryForChain(chainID)
	if err != nil {
		return common.Address{}, err
	}
	salt := crypto.Keccak256Hash(eoaAddress.Bytes())
	return create2(factory, salt, common.HexToHash(proxyInitCodeHash)), nil
}

func create2(factory common.Address, salt, initCodeHash common.Hash) common.Address {
	input := make([]byte, 1+20+32+32)
	input[0] = 0xff
	copy(input[1:21], factory.Bytes())
	copy(input[21:53], salt.Bytes())
	copy(input[53:85], initCodeHash.Bytes())
	hash := crypto.Keccak256Hash(input)
	var addr common.Address
	copy(addr[:], hash[12:])
	return addr
}

func safeFactoryForChain(chainID int) (common.Address, error) {
	switch chainID {
	case 137:
		return common.HexToAddress(polygonSafeFactory), nil
	default:
		return common.Address{}, fmt.Errorf("safe wallet derivation not supported on chain %d", chainID)
	}
}

func proxyFactoryForChain(chainID int) (common.Address, error) {
	switch chainID {
	case 137:
		return common.HexToAddress(polygonProxyFactory), nil
	default:
		return common.Address{}, fmt.Errorf("proxy wallet derivation not supported on chain %d", chainID)
	}
}

func hashString(s string) common.Hash {
	return crypto.Keccak256Hash([]byte(s))
}
