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

    if (!$req)       { echo '{"error":"invalid json"}';  exit; }
    $token = $req['token'] ?? '';
    if (!$token)     { echo '{"error":"token required"}'; exit; }

    $store = [];
    if (file_exists($DATA_FILE)) {
        $store = json_decode(file_get_contents($DATA_FILE), true) ?? [];
    }

    if (!isset($store[$token])) {
        $store[$token] = ['usage' => [], 'deletedHours' => []];
    }

    // 旧データ形式（usage/deletedHours キーなし、日付が直接並んでいる）を新形式に移行
    if (!array_key_exists('usage', $store[$token])) {
        $store[$token] = ['usage' => $store[$token], 'deletedHours' => []];
    }

    // クライアントの usage をマージ（各時間帯の最大値を採用）
    foreach ($req['usage'] ?? [] as $date => $hours) {
        if (!is_array($hours) || count($hours) !== 24) continue;
        if (!isset($store[$token]['usage'][$date])) {
            $store[$token]['usage'][$date] = array_fill(0, 24, 0);
        }
        for ($h = 0; $h < 24; $h++) {
            $store[$token]['usage'][$date][$h] = max(
                $store[$token]['usage'][$date][$h],
                $hours[$h] ?? 0
            );
        }
    }

    // クライアントの deletedHours をサーバーの削除リストに累積（全端末の union）
    foreach ($req['deletedHours'] ?? [] as $date => $hours) {
        if (!isset($store[$token]['deletedHours'][$date])) {
            $store[$token]['deletedHours'][$date] = [];
        }
        $merged = array_unique(array_merge(
            $store[$token]['deletedHours'][$date],
            array_map('intval', $hours)
        ));
        $store[$token]['deletedHours'][$date] = array_values($merged);
    }

    // 累積された全端末の deletedHours を usage に適用（max マージより優先）
    foreach ($store[$token]['deletedHours'] as $date => $hours) {
        if (!isset($store[$token]['usage'][$date])) continue;
        foreach ($hours as $h) {
            if ($h >= 0 && $h < 24) {
                $store[$token]['usage'][$date][$h] = 0;
            }
        }
        if (array_sum($store[$token]['usage'][$date]) === 0) {
            unset($store[$token]['usage'][$date]);
        }
    }

    file_put_contents($DATA_FILE, json_encode($store));

    // deletedHours も返すことでクライアントが他端末の削除情報を受け取れる
    echo json_encode([
        'usage'        => $store[$token]['usage'],
        'deletedHours' => $store[$token]['deletedHours'],
    ]);
    exit;
}

echo json_encode(['error' => 'not found', 'path' => $path]);
