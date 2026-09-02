$ErrorActionPreference = "Stop"
$base = "http://localhost:8080"
$user = "accept-$([guid]::NewGuid().ToString('N').Substring(0,8))"
$pngB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
$file = Join-Path $env:TEMP "gophprofile-e2e.png"
[IO.File]::WriteAllBytes($file, [Convert]::FromBase64String($pngB64))

function Assert-Curl([string]$name, [int]$want, [string[]]$curlArgs) {
    $tmp = Join-Path $env:TEMP "gophprofile-e2e-out.txt"
    $code = & curl.exe -sS -o $tmp -w "%{http_code}" @curlArgs
    $body = Get-Content $tmp -Raw -ErrorAction SilentlyContinue
    if ([int]$code -ne $want) {
        throw "${name}: expected ${want} got ${code} body=${body}"
    }
    return $body
}

Assert-Curl "health" 200 @("$base/health") | Out-Null
Assert-Curl "upload page" 200 @("$base/web/upload") | Out-Null

$upload = Assert-Curl "upload" 201 @(
    "-H", "X-User-ID: $user",
    "-F", "file=@$file;type=image/png",
    "$base/api/v1/avatars"
)
$created = $upload | ConvertFrom-Json
$id = $created.id
Write-Host "uploaded $id"

Start-Sleep -Seconds 4

Assert-Curl "metadata" 200 @("$base/api/v1/avatars/$id/metadata") | Out-Null
Assert-Curl "get avatar" 200 @("$base/api/v1/avatars/$id") | Out-Null
Assert-Curl "list" 200 @("$base/api/v1/users/$user/avatars") | Out-Null
Assert-Curl "current" 200 @("$base/api/v1/users/$user/avatar") | Out-Null
Assert-Curl "gallery" 200 @("$base/web/gallery/$user") | Out-Null
Assert-Curl "delete" 204 @("-X", "DELETE", "-H", "X-User-ID: $user", "$base/api/v1/avatars/$id") | Out-Null

Write-Host "E2E OK user=$user id=$id"
