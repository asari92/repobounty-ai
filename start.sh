#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
DIM='\033[2m'
BOLD='\033[1m'
NC='\033[0m'

# Create .env from example if missing
if [ ! -f .env ]; then
  cp .env.example .env
  echo -e "${CYAN}Created .env from .env.example${NC}"
  echo ""
fi

# Load .env
set -a
source .env
set +a

# ── Check variables ──────────────────────────────────────────────

missing=()    # blocks startup
warnings=()   # works without, but degraded

# Required
if [ -z "${JWT_SECRET:-}" ] || [ "${JWT_SECRET}" = "change-me-to-a-random-string-at-least-32-chars" ]; then
  missing+=("JWT_SECRET")
fi

# Recommended — grouped by feature
if [ -z "${GITHUB_TOKEN:-}" ]; then
  warnings+=("GITHUB_TOKEN")
fi
if [ -z "${GITHUB_CLIENT_ID:-}" ]; then
  warnings+=("GITHUB_CLIENT_ID")
fi
if [ -z "${GITHUB_CLIENT_SECRET:-}" ]; then
  warnings+=("GITHUB_CLIENT_SECRET")
fi
if [ -z "${SERVICE_PRIVATE_KEY:-}" ]; then
  warnings+=("SERVICE_PRIVATE_KEY")
fi
if [ -z "${OPENROUTER_API_KEY:-}" ]; then
  warnings+=("OPENROUTER_API_KEY")
fi

# ── Print report ─────────────────────────────────────────────────

has_issues=false

if [ ${#missing[@]} -gt 0 ] || [ ${#warnings[@]} -gt 0 ]; then
  echo -e "${BOLD}Environment check:${NC}"
  echo ""
fi

if [ ${#missing[@]} -gt 0 ]; then
  has_issues=true
  echo -e "  ${RED}Missing (required):${NC}"
  for var in "${missing[@]}"; do
    case "$var" in
      JWT_SECRET)
        echo -e "    ${RED}✗${NC} JWT_SECRET"
        echo -e "      ${DIM}Random string, min 32 chars. Generate:${NC}"
        echo -e "      ${CYAN}openssl rand -base64 32${NC}"
        ;;
    esac
  done
  echo ""
fi

if [ ${#warnings[@]} -gt 0 ]; then
  echo -e "  ${YELLOW}Missing (optional — app runs in mock/demo mode without these):${NC}"
  for var in "${warnings[@]}"; do
    case "$var" in
      GITHUB_TOKEN)
        echo -e "    ${YELLOW}○${NC} GITHUB_TOKEN"
        echo -e "      ${DIM}GitHub Personal Access Token for fetching contributor data${NC}"
        echo -e "      ${DIM}Create: https://github.com/settings/tokens → scopes: repo, read:user${NC}"
        ;;
      GITHUB_CLIENT_ID)
        echo -e "    ${YELLOW}○${NC} GITHUB_CLIENT_ID"
        echo -e "      ${DIM}GitHub OAuth App client ID (needed for user login)${NC}"
        echo -e "      ${DIM}Create: https://github.com/settings/developers → New OAuth App${NC}"
        echo -e "      ${DIM}Callback URL: ${FRONTEND_URL:-http://localhost:5173}/auth/callback${NC}"
        ;;
      GITHUB_CLIENT_SECRET)
        echo -e "    ${YELLOW}○${NC} GITHUB_CLIENT_SECRET"
        echo -e "      ${DIM}GitHub OAuth App client secret (same app as GITHUB_CLIENT_ID)${NC}"
        ;;
      SERVICE_PRIVATE_KEY)
        echo -e "    ${YELLOW}○${NC} SERVICE_PRIVATE_KEY"
        echo -e "      ${DIM}Backend service keypair for on-chain transactions${NC}"
        echo -e "      ${DIM}Generate: solana-keygen new -o authority.json${NC}"
        echo -e "      ${DIM}Then paste contents (JSON array) or base58 private key${NC}"
        ;;
      OPENROUTER_API_KEY)
        echo -e "    ${YELLOW}○${NC} OPENROUTER_API_KEY"
        echo -e "      ${DIM}For AI-based allocation (without it: deterministic fallback)${NC}"
        echo -e "      ${DIM}Get key: https://openrouter.ai/keys${NC}"
        ;;
    esac
  done
  echo ""
fi

# Block on missing required vars
if [ ${#missing[@]} -gt 0 ]; then
  echo -e "  ${RED}Fix required variables in .env, then re-run ./start.sh${NC}"
  exit 1
fi

# ── Start ────────────────────────────────────────────────────────

echo -e "${GREEN}Starting RepoBounty AI...${NC}"
echo ""
echo -e "  Frontend : ${CYAN}http://localhost:5173${NC}"
echo -e "  Backend  : ${CYAN}http://localhost:8080${NC}"
echo -e "  Health   : ${CYAN}http://localhost:8080/api/health${NC}"
echo -e "  Compose  : ${DIM}frontend + backend only${NC}"
echo ""
echo -e "  ${DIM}If you need to build/test/deploy the Solana program in Docker, run:${NC}"
echo -e "  ${CYAN}docker compose --profile deploy run --rm solana-check${NC}"
echo -e "  ${CYAN}docker compose --profile deploy run --rm solana-deployer${NC}"
echo -e "  ${DIM}deploy uses SOLANA_DEPLOY_WALLET as admin wallet and SERVICE_PRIVATE_KEY as service wallet${NC}"
echo ""

docker compose up --build "$@"
