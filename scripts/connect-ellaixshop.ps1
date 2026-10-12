# Configure an ELLAIXSHOP supplier connection and optionally import draft goods.
# This script never creates a supplier purchase or publishes a local product.
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)][string]$AdminBaseUrl,
    [ValidateRange(0, 1000)][double]$MarkupPercent = 0,
    [uint32[]]$UpstreamProductIds = @(),
    [switch]$ImportDrafts
)

$ErrorActionPreference = 'Stop'
if ([string]::IsNullOrWhiteSpace($env:ZXA_ADMIN_TOKEN) -or [string]::IsNullOrWhiteSpace($env:ELLAIXSHOP_MERCHANT_SECRET)) {
    throw 'Set ZXA_ADMIN_TOKEN and ELLAIXSHOP_MERCHANT_SECRET in the current process before running this script.'
}
$adminUri = [Uri]$AdminBaseUrl
if (-not $adminUri.IsAbsoluteUri -or $adminUri.UserInfo -or $adminUri.Query -or $adminUri.Fragment -or
    ($adminUri.Scheme -ne 'https' -and -not ($adminUri.Scheme -eq 'http' -and $adminUri.IsLoopback))) {
    throw 'Use the HTTPS backend URL, or HTTP on localhost, without credentials, query or fragment.'
}
$apiBase = $AdminBaseUrl.TrimEnd('/')
if (-not $apiBase.EndsWith('/api/v1')) { $apiBase += '/api/v1' }
$adminHeaders = @{ Authorization = 'Bearer ' + $env:ZXA_ADMIN_TOKEN }

function Invoke-ShopApi {
    param([string]$Method, [string]$Path, [object]$Body)
    $requestOptions = @{
        Uri = $apiBase + $Path
        Method = $Method
        Headers = $adminHeaders
        MaximumRedirection = 0
        TimeoutSec = 120
        ErrorAction = 'Stop'
    }
    if ($null -ne $Body) {
        $requestOptions.ContentType = 'application/json; charset=utf-8'
        $requestOptions.Body = [Text.Encoding]::UTF8.GetBytes(($Body | ConvertTo-Json -Depth 10 -Compress))
    }
    try { $response = Invoke-RestMethod @requestOptions }
    catch { throw "Backend request failed: $Method $Path. Check the URL, login token and backend logs." }
    if ($null -eq $response.status_code -or $response.status_code -ne 0) {
        throw "Backend rejected $Method $Path (status_code=$($response.status_code))."
    }
    return $response
}

$matchingConnections = @()
$page = 1
do {
    $listing = Invoke-ShopApi -Method GET -Path "/admin/site-connections?page=$page&page_size=100"
    $matchingConnections += @($listing.data | Where-Object {
        $_.base_url.TrimEnd('/') -eq 'https://ellaixshop.com' -and $_.api_key -eq '1039'
    })
    $page++
} while ($page -le $listing.pagination.total_page)
if ($matchingConnections.Count -gt 1) { throw 'Multiple connections already exist for this merchant. Review them in the admin panel.' }
if ($matchingConnections.Count -eq 1) {
    $connection = $matchingConnections[0]
    if ($connection.protocol -ne 'acg-faka') { throw 'The existing connection uses another protocol. Edit it to Acg-Faka in the admin panel.' }
} else {
    $created = Invoke-ShopApi -Method POST -Path '/admin/site-connections' -Body @{
        name = 'ELLAIXSHOP'
        base_url = 'https://ellaixshop.com'
        api_key = '1039'
        api_secret = $env:ELLAIXSHOP_MERCHANT_SECRET
        protocol = 'acg-faka'
        exchange_rate = 1
        price_markup_percent = $MarkupPercent
        price_rounding_mode = 'none'
        auto_sync_price = $true
    }
    $connection = $created.data
}
$connectionId = [uint32]$connection.id
$ping = Invoke-ShopApi -Method POST -Path "/admin/site-connections/$connectionId/ping"

$importResult = $null
if ($ImportDrafts) {
    if ($UpstreamProductIds.Count -eq 0) { throw 'Supply UpstreamProductIds when importing draft products.' }
    $imported = Invoke-ShopApi -Method POST -Path '/admin/product-mappings/batch-import' -Body @{
        connection_id = $connectionId
        upstream_product_ids = @($UpstreamProductIds)
        auto_create_category = $true
    }
    $importResult = $imported.data
}
[ordered]@{
    connection_id = $connectionId
    site = $ping.data.site_name
    merchant_id = $ping.data.user_id
    balance = $ping.data.balance
    currency = $ping.data.currency
    imports = $importResult
    purchases_created = 0
    products_published = 0
} | ConvertTo-Json -Depth 10
