<#
.SYNOPSIS
  Send one prompt to a running Mini AI-DOS and print the answer.

.DESCRIPTION
  Sends the body as UTF-8 (so Arabic and other non-ASCII text survive
  the Windows console) and prints only the assistant's reply, plus a
  one-line token summary. Omit -Model to use the gateway's AI_MODEL.

.EXAMPLE
  .\scripts\ask.ps1 "Write a Go table-driven test for a clampLimit(int) int function"
.EXAMPLE
  .\scripts\ask.ps1 "اشرح الفرق بين goroutine و thread" -System "أجب بالعربية فقط"
.EXAMPLE
  .\scripts\ask.ps1 "Summarize this file" -File .\README.md -Model llama3.1:8b
#>
param(
    [Parameter(Mandatory = $true, Position = 0)][string]$Prompt,
    [string]$System = "",
    [string]$Model = "",
    [string]$File = "",
    [int]$MaxTokens = 1024,
    [string]$ApiKey = $(if ($env:MINI_AI_DOS_API_KEY) { $env:MINI_AI_DOS_API_KEY } else { "dev-key" }),
    [string]$Url = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

$content = $Prompt
if ($File) {
    $content += "`n`n--- $File ---`n" + (Get-Content -Raw -Encoding UTF8 $File)
}

$messages = @()
if ($System) { $messages += @{ role = "system"; content = $System } }
$messages += @{ role = "user"; content = $content }

$body = @{ messages = $messages; max_tokens = $MaxTokens }
if ($Model) { $body.model = $Model }

$bytes = [System.Text.Encoding]::UTF8.GetBytes(($body | ConvertTo-Json -Depth 6 -Compress))
try {
    $resp = Invoke-RestMethod -Uri "$Url/v1/chat/completions" -Method Post `
        -Headers @{ Authorization = "Bearer $ApiKey" } `
        -ContentType "application/json; charset=utf-8" -Body $bytes -TimeoutSec 300
} catch {
    $detail = ""
    if ($_.Exception.Response) {
        $reader = New-Object IO.StreamReader($_.Exception.Response.GetResponseStream())
        $detail = $reader.ReadToEnd()
    }
    throw "Request failed: $($_.Exception.Message) $detail"
}

Write-Output $resp.choices[0].message.content
Write-Host ("-- model={0} tokens={1} (prompt {2}, completion {3})" -f $resp.model, $resp.usage.total_tokens, $resp.usage.prompt_tokens, $resp.usage.completion_tokens) -ForegroundColor DarkGray
