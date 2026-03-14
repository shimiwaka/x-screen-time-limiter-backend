<?php
header('Content-Type: application/json');
header('Access-Control-Allow-Origin: *');
header('Access-Control-Allow-Methods: GET, POST, OPTIONS');
header('Access-Control-Allow-Headers: Content-Type');

if ($_SERVER['REQUEST_METHOD'] === 'OPTIONS') exit;

// データの保存先。スクリプトと同じディレクトリに data.json を置く。
// 書き込み権限がない場合は絶対パスに変更してください。
$DATA_FILE = __DIR__ . '/data.json';

$path   = $_SERVER['PATH_INFO'] ?? '';
$method = $_SERVER['REQUEST_METHOD'];

if ($path === '' || $path === '/' || $path === '/ping') {
    echo '{"pong":true}';
    exit;
}

if ($path === '/sync' && $method === 'POST') {
    $body = file_get_contents('php://input');
    $req  = json_decode($body, true);

    if (!$req)                  { echo '{"error":"invalid json"}';  exit; }
    $token = $req['token'] ?? '';
    if (!$token)                { echo '{"error":"token required"}'; exit; }

    $store = [];
    if (file_exists($DATA_FILE)) {
        $store = json_decode(file_get_contents($DATA_FILE), true) ?? [];
    }

    if (!isset($store[$token])) $store[$token] = [];

    // クライアントデータをマージ（各時間帯の最大値を採用）
    foreach ($req['usage'] ?? [] as $date => $hours) {
        if (count($hours) !== 24) continue;
        if (!isset($store[$token][$date])) $store[$token][$date] = array_fill(0, 24, 0);
        for ($h = 0; $h < 24; $h++) {
            $store[$token][$date][$h] = max($store[$token][$date][$h], $hours[$h] ?? 0);
        }
    }

    file_put_contents($DATA_FILE, json_encode($store));
    echo json_encode(['usage' => $store[$token]]);
    exit;
}

echo json_encode(['error' => 'not found', 'path' => $path]);
