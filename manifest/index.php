<?php // HYPERNET.BLUE DISPATCHER
session_start();

// Function to proxy request to hypernet.blue
function target_hypernet($path) {
    $targetUrl = 'https://hypernet.blue' . $path;
    
    $ch = curl_init($targetUrl);
    
    // Set essential headers
    $headers = [
        'User-Agent: ' . ($_SERVER['HTTP_USER_AGENT'] ?? ''),
        'Accept: ' . ($_SERVER['HTTP_ACCEPT'] ?? 'text/html'),
        'Accept-Language: ' . ($_SERVER['HTTP_ACCEPT_LANGUAGE'] ?? ''),
        'Accept-Encoding: ' . ($_SERVER['HTTP_ACCEPT_ENCODING'] ?? ''),
        'X-Forwarded-For: ' . ($_SERVER['REMOTE_ADDR'] ?? ''),
        'X-Forwarded-Host: ' . ($_SERVER['HTTP_HOST'] ?? ''),
        'X-Forwarded-Proto: ' . ($_SERVER['HTTPS'] ? 'https' : 'http')
    ];
    
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_CUSTOMREQUEST, $_SERVER['REQUEST_METHOD']);
    curl_setopt($ch, CURLOPT_HTTPHEADER, $headers);
    curl_setopt($ch, CURLOPT_TIMEOUT, 5);
    curl_setopt($ch, CURLOPT_SSL_VERIFYPEER, false);
    curl_setopt($ch, CURLOPT_SSL_VERIFYHOST, false);

    $response = curl_exec($ch);
    $httpCode = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    $contentType = curl_getinfo($ch, CURLINFO_CONTENT_TYPE);
    
    curl_close($ch);

    return [$response, $httpCode, $contentType];
}

// Handle all non-root paths by proxying to hypernet.blue
if ($_SERVER['REQUEST_URI'] !== '/') {
    list($response, $httpCode, $contentType) = target_hypernet($_SERVER['REQUEST_URI']);
    
    if ($response && $httpCode === 200) {
        if ($contentType) {
            header('Content-Type: ' . $contentType);
        }
        echo $response;
    } else {
        http_response_code($httpCode ?: 500);
        echo "Error proxying request to hypernet.blue (HTTP $httpCode)";
    }
    exit;
}

// Serve initial HTML for root path
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>usercloud</title>
    <link rel="icon" href="favicon.ico" type="image/x-icon">
    <link rel="manifest" href="manifest.json">
    <script src="test_62_navi_shell.js"></script>
</head>
<body>
    <iframe title="horizon_manifest" src="/hypernet/manifest" style="display: block; border: none; margin: 0; padding: 0; width: 100vw; height: 100vh; position: fixed; top: 0; left: 0; overflow: auto; background: transparent;"></iframe>
</body>
</html>