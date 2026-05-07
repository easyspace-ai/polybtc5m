/**
 * One-time script to generate Polymarket CLOB API credentials.
 *
 * Usage:
 *   # Deposit wallet (POLY_1271 — new API users):
 *   POLYMARKET_PRIVATE_KEY=0x... POLYMARKET_PROXY_WALLET=0x... POLYMARKET_SIG_TYPE=3 node scripts/gen-api-keys.mjs
 *
 *   # POLY_PROXY legacy (email/social login):
 *   POLYMARKET_PRIVATE_KEY=0x... POLYMARKET_PROXY_WALLET=0x... POLYMARKET_SIG_TYPE=1 node scripts/gen-api-keys.mjs
 *
 *   # Gnosis Safe / browser wallet:
 *   POLYMARKET_PRIVATE_KEY=0x... POLYMARKET_PROXY_WALLET=0x... POLYMARKET_SIG_TYPE=2 node scripts/gen-api-keys.mjs
 *
 *   # EOA only:
 *   POLYMARKET_PRIVATE_KEY=0x... node scripts/gen-api-keys.mjs
 */

import { config as dotenvConfig } from 'dotenv';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';
import { bootstrap } from 'global-agent';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

// Load .env from the project root (parent of scripts/)
dotenvConfig({ path: resolve(__dirname, '..', '.env'), override: true });

// ── Proxy setup ─────────────────────────────────────────────────────────────
const proxyUrl = process.env.PROXY_URL || process.env.HTTP_PROXY || process.env.HTTPS_PROXY;
if (proxyUrl) {
  process.env.GLOBAL_AGENT_HTTP_PROXY = proxyUrl;
  process.env.GLOBAL_AGENT_HTTPS_PROXY = proxyUrl;
  bootstrap();
  console.log(`🌐 Proxy enabled: ${proxyUrl}\n`);
}

import { ClobClient, SignatureTypeV2 } from "@polymarket/clob-client-v2";
import { createWalletClient, http } from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { polygon } from "viem/chains";

const PRIVATE_KEY = process.env.POLYMARKET_PRIVATE_KEY;
const PROXY_WALLET = process.env.POLYMARKET_PROXY_WALLET;
const SIG_TYPE = process.env.POLYMARKET_SIG_TYPE ? parseInt(process.env.POLYMARKET_SIG_TYPE, 10) : 0;

if (!PRIVATE_KEY) {
  console.error("Error: POLYMARKET_PRIVATE_KEY environment variable is not set.");
  console.error("\nUsage:");
  console.error("  node scripts/gen-api-keys.mjs   (reads from .env)");
  process.exit(1);
}

if (!PRIVATE_KEY.startsWith("0x") || PRIVATE_KEY.length !== 66) {
  console.error("Error: Private key must start with 0x and be 66 characters total.");
  process.exit(1);
}

const account = privateKeyToAccount(PRIVATE_KEY);
const signer = createWalletClient({ account, chain: polygon, transport: http() });

const clientConfig = {
  host: "https://clob.polymarket.com",
  chain: 137,
  signer,
};

if (SIG_TYPE === 3) {
  if (!PROXY_WALLET) {
    console.error("Error: POLYMARKET_PROXY_WALLET must be set for POLY_1271 (sig type 3).");
    process.exit(1);
  }
  console.log(`🔑 Type 3 (POLY_1271) — Deposit wallet: ${PROXY_WALLET}`);
  clientConfig.funderAddress = PROXY_WALLET;
  clientConfig.signatureType = SignatureTypeV2.POLY_1271;
} else if (SIG_TYPE === 1) {
  if (!PROXY_WALLET) {
    console.error("Error: POLYMARKET_PROXY_WALLET must be set for POLY_PROXY (sig type 1).");
    process.exit(1);
  }
  console.log(`🔑 Type 1 (POLY_PROXY) — Proxy: ${PROXY_WALLET}`);
  clientConfig.funderAddress = PROXY_WALLET;
  clientConfig.signatureType = SignatureTypeV2.POLY_PROXY;
} else if (SIG_TYPE === 2) {
  if (!PROXY_WALLET) {
    console.error("Error: POLYMARKET_PROXY_WALLET must be set for GNOSIS_SAFE (sig type 2).");
    process.exit(1);
  }
  // For type 2, the API key is tied to the EOA signer (not the Safe address).
  // The EOA signs on behalf of the Safe; Polymarket verifies ownership off-chain.
  // Do NOT set funderAddress here — it would bind the API key to the Safe address,
  // causing "order signer address has to be the address of the API KEY" errors.
  console.log(`🔑 Type 2 (GNOSIS_SAFE) — Safe: ${PROXY_WALLET}`);
  console.log(`   API key will be registered to EOA signer: ${account.address}`);
  clientConfig.signatureType = SignatureTypeV2.POLY_GNOSIS_SAFE;
} else {
  console.log(`🔑 Type 0 (EOA) — Address: ${account.address}`);
}

const client = new ClobClient(clientConfig);

let creds;
try {
  creds = await client.createOrDeriveApiKey();
} catch (err) {
  console.error("\n❌ Failed to generate credentials:", err.message ?? err);
  if (err.response?.data) {
    console.error("   Server response:", JSON.stringify(err.response.data));
  }
  process.exit(1);
}

console.log("\n========== Add these to your .env ==========\n");
console.log(`POLYMARKET_API_KEY=${creds.key}`);
console.log(`POLYMARKET_API_SECRET=${creds.secret}`);
console.log(`POLYMARKET_API_PASSPHRASE=${creds.passphrase}`);
console.log("\n============================================");
console.log(`\n✅ API key registered to: ${account.address}`);
if (PROXY_WALLET && SIG_TYPE !== 0) {
  console.log(`   Proxy / Safe / Funder: ${PROXY_WALLET}`);
}
console.log("\nDone. Save these now — the secret and passphrase cannot be retrieved again.");
