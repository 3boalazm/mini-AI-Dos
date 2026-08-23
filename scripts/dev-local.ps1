<#
.SYNOPSIS
  Run Mini AI-DOS locally against an Ollama model (or the mock provider).

.DESCRIPTION
  Sets the gateway's environment for one run and starts it in the
  foreground. Ollama exposes an OpenAI-compatible API, so the gateway's
  "openai" provider talks to it unchanged. Ctrl+C stops the gateway
  gracefully.

.EXAMPLE
  .\scripts\dev-local.ps1                          # qwen2.5-coder:7b
.EXAMPLE
  .\scripts\dev-local.ps1 -Model llama3.1:8b
.EXAMPLE
  .\scripts\dev-local.ps1 -Model mock              # no Ollama needed
#>
param(
    [string]$Model = "qwen2.5-coder:7b",
    [string]$ApiKey = "dev-key",
    [int]$Port = 8080,
    [string]$OllamaUrl = "http://localhost:11434"
)

$ErrorActionPreference = "Stop"
Set-Location (Split-Path $PSScriptRoot -Parent)

$env:API_KEY_AUTH_MODE = "env"
$env:MINI_AI_DOS_API_KEY = $ApiKey
$env:GATEWAY_PORT = "$Port"

if ($Model -eq "mock") {
    $env:AI_PROVIDER = "mock"
} else {
    try {
        $tags = Invoke-RestMethod "$OllamaUrl/api/tags" -TimeoutSec 3
    } catch {
        throw "Ollama is not reachable at $OllamaUrl - start the Ollama app first."
    }
    if (-not ($tags.models.name -contains $Model)) {
        $have = ($tags.models.name | Where-Object { $_ }) -join ", "
        throw "Model '$Model' is not pulled (have: $have). Run: ollama pull $Model"
    }
    $env:AI_PROVIDER = "openai"
    $env:AI_BASE_URL = "$OllamaUrl/v1"
    # Ollama ignores the key, but the gateway requires one for the openai provider.
    $env:AI_API_KEY = "ollama"
    $env:AI_MODEL = $Model
}

Write-Host "Mini AI-DOS  provider=$($env:AI_PROVIDER)  model=$Model  port=$Port  key=$ApiKey" -ForegroundColor Cyan
go run ./services/gateway/cmd/gateway
