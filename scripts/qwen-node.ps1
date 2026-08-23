<#
.SYNOPSIS
  Expose this PC's Qwen (via the Mini AI-DOS gateway) to the internet
  through a Cloudflare quick tunnel, so the Railway deployment can use
  it as one upstream in its failover chain.

.DESCRIPTION
  Starts a dedicated gateway instance that serves ONLY the local Ollama
  model (no failover here — Railway owns the failover), protected by
  the node API key, then opens a Cloudflare quick tunnel to it. The
  tunnel URL is printed by cloudflared — put it in Railway's
  AI_PROVIDERS entry as base_url (append /v1).

  Quick tunnels get a NEW random URL on every start — fine for testing;
  for a stable hostname create a named tunnel on a Cloudflare-managed
  domain. Ctrl+C stops the tunnel and the gateway.

.EXAMPLE
  .\scripts\qwen-node.ps1 -ApiKey "<the node key configured in Railway>"
.EXAMPLE
  .\scripts\qwen-node.ps1 -ApiKey "..." -Model qwen2.5-coder:7b
#>
param(
    [Parameter(Mandatory = $true)][string]$ApiKey,
    [string]$Model = "qwen2.5:3b",
    [int]$Port = 8095,
    [string]$OllamaUrl = "http://localhost:11434"
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

try {
    $tags = Invoke-RestMethod "$OllamaUrl/api/tags" -TimeoutSec 3
} catch {
    throw "Ollama is not reachable at $OllamaUrl - start the Ollama app first."
}
if (-not ($tags.models.name -contains $Model)) {
    throw "Model '$Model' is not pulled. Run: ollama pull $Model"
}

Write-Host "Building gateway..." -ForegroundColor DarkGray
go build -o mini-ai-dos-node.exe ./services/gateway/cmd/gateway
if ($LASTEXITCODE -ne 0) { throw "go build failed" }

# Environment is inherited at spawn time (Windows PowerShell 5.1 has
# no Start-Process -Environment). These affect only this script's
# process and the child it spawns.
$env:API_KEY_AUTH_MODE = "env"
$env:MINI_AI_DOS_API_KEY = $ApiKey
$env:GATEWAY_PORT = "$Port"
$env:AI_PROVIDER = "openai"
$env:AI_BASE_URL = "$OllamaUrl/v1"
$env:AI_API_KEY = "ollama"
$env:AI_MODEL = $Model
$env:APP_ENV = "production"
$env:LOG_LEVEL = "info"
$env:AI_PROVIDERS = ""   # never inherit a failover chain into this node

$gw = Start-Process -PassThru -WindowStyle Hidden -FilePath ".\mini-ai-dos-node.exe"
Start-Sleep -Seconds 2
if ($gw.HasExited) { throw "gateway failed to start (port $Port already in use?)" }
Write-Host "Qwen node up: gateway pid $($gw.Id) on :$Port -> $Model" -ForegroundColor Cyan
Write-Host "Opening Cloudflare quick tunnel - copy the https://....trycloudflare.com URL below" -ForegroundColor Cyan
Write-Host "Railway AI_PROVIDERS entry base_url = that URL + /v1" -ForegroundColor Cyan

try {
    cloudflared tunnel --url "http://localhost:$Port"
} finally {
    Stop-Process -Id $gw.Id -Force -ErrorAction SilentlyContinue
    Remove-Item ".\mini-ai-dos-node.exe" -ErrorAction SilentlyContinue
    Write-Host "Qwen node stopped." -ForegroundColor Yellow
}
