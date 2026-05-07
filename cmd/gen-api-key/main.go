package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/silver/pmvibes/internal/config"
	"github.com/silver/pmvibes/pkg/polymarket"
)

const clobBaseURL = "https://clob.polymarket.com"

type apiKeyResponse struct {
	ApiKey     string `json:"apiKey"`
	Secret     string `json:"secret"`
	Passphrase string `json:"passphrase"`
}

func main() {
	config.LoadDotEnv()

	privateKey := strings.TrimSpace(os.Getenv("POLYMARKET_PRIVATE_KEY"))
	if privateKey == "" {
		log.Fatal("❌ POLYMARKET_PRIVATE_KEY is required")
	}
	if !strings.HasPrefix(privateKey, "0x") || len(privateKey) != 66 {
		log.Fatal("❌ Private key must start with 0x and be 66 characters total")
	}

	pkHex := privateKey[2:]
	pk, err := crypto.HexToECDSA(pkHex)
	if err != nil {
		log.Fatalf("❌ Invalid private key: %v", err)
	}

	eoaAddr := crypto.PubkeyToAddress(pk.PublicKey)
	fmt.Printf("🔑 EOA Address: %s\n", eoaAddr.Hex())

	sigTypeStr := os.Getenv("POLYMARKET_SIG_TYPE")
	sigType, _ := strconv.Atoi(sigTypeStr)
	if sigType == 0 {
		sigType = 2 // default for browser-wallet legacy users
	}

	proxyWallet := strings.TrimSpace(os.Getenv("POLYMARKET_PROXY_WALLET"))

	switch sigType {
	case 2:
		fmt.Println("🔑 Signature Type: 2 (GNOSIS_SAFE / Browser Wallet)")
		derivedSafe, err := polymarket.DeriveSafeWallet(eoaAddr, 137)
		if err != nil {
			log.Fatalf("❌ Failed to derive Safe wallet: %v", err)
		}
		fmt.Printf("   Derived Safe:    %s\n", derivedSafe.Hex())
		if proxyWallet != "" && !strings.EqualFold(proxyWallet, derivedSafe.Hex()) {
			fmt.Printf("   ⚠️  Warning: .env POLYMARKET_PROXY_WALLET differs from derived address:\n")
			fmt.Printf("      .env:     %s\n", proxyWallet)
			fmt.Printf("      derived:  %s\n", derivedSafe.Hex())
			fmt.Printf("   → Order maker will use .env value; ensure it holds USDC.e.\n")
		} else if proxyWallet == "" {
			fmt.Printf("   → Set POLYMARKET_PROXY_WALLET=%s in your .env\n", derivedSafe.Hex())
		}
	case 1:
		fmt.Println("🔑 Signature Type: 1 (POLY_PROXY / Magic Link)")
		derivedProxy, err := polymarket.DeriveProxyWallet(eoaAddr, 137)
		if err != nil {
			log.Fatalf("❌ Failed to derive Proxy wallet: %v", err)
		}
		fmt.Printf("   Derived Proxy:   %s\n", derivedProxy.Hex())
		if proxyWallet != "" && !strings.EqualFold(proxyWallet, derivedProxy.Hex()) {
			fmt.Printf("   ⚠️  Warning: .env POLYMARKET_PROXY_WALLET differs from derived address.\n")
		} else if proxyWallet == "" {
			fmt.Printf("   → Set POLYMARKET_PROXY_WALLET=%s in your .env\n", derivedProxy.Hex())
		}
	case 0:
		fmt.Println("🔑 Signature Type: 0 (EOA / MetaMask self-custody)")
	default:
		log.Fatalf("❌ Unsupported signature type: %d", sigType)
	}

	// Generate L1 auth headers
	headers, err := polymarket.CreateL1Headers(pk, 137, 0)
	if err != nil {
		log.Fatalf("❌ Failed to create L1 headers: %v", err)
	}

	fmt.Printf("\n📡 Requesting API key from CLOB...\n")

	httpClient := polymarket.NewHTTPClientWithProxy(30 * time.Second)
	ctx := context.Background()

	// Strategy: try POST /auth/api-key first, fall back to GET /auth/derive-api-key
	var creds *apiKeyResponse
	creds, err = createAPIKey(ctx, httpClient, headers)
	if err != nil {
		fmt.Printf("   POST /auth/api-key failed: %v\n", err)
		fmt.Printf("   Trying GET /auth/derive-api-key...\n")
		creds, err = deriveAPIKey(ctx, httpClient, headers)
		if err != nil {
			log.Fatalf("❌ Failed to derive API key: %v", err)
		}
	}

	fmt.Println("\n========== Add these to your .env ==========\n")
	fmt.Printf("POLYMARKET_API_KEY=%s\n", creds.ApiKey)
	fmt.Printf("POLYMARKET_API_SECRET=%s\n", creds.Secret)
	fmt.Printf("POLYMARKET_API_PASSPHRASE=%s\n", creds.Passphrase)
	fmt.Println("\n============================================")
	fmt.Printf("\n✅ API key bound to EOA: %s\n", eoaAddr.Hex())
	fmt.Println("\n💾 Save these now — the secret and passphrase cannot be retrieved again.")
}

func createAPIKey(ctx context.Context, client *http.Client, headers map[string]string) (*apiKeyResponse, error) {
	return doAuthRequest(ctx, client, http.MethodPost, "/auth/api-key", headers)
}

func deriveAPIKey(ctx context.Context, client *http.Client, headers map[string]string) (*apiKeyResponse, error) {
	return doAuthRequest(ctx, client, http.MethodGet, "/auth/derive-api-key", headers)
}

func doAuthRequest(ctx context.Context, client *http.Client, method, path string, headers map[string]string) (*apiKeyResponse, error) {
	req, err := http.NewRequestWithContext(ctx, method, clobBaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		if msg, ok := body["error"].(string); ok && msg != "" {
			return nil, fmt.Errorf("CLOB %d: %s", resp.StatusCode, msg)
		}
		return nil, fmt.Errorf("CLOB %d", resp.StatusCode)
	}

	var result apiKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
