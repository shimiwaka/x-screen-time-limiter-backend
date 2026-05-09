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
        $store[$token] = ['usage' => [], 'deletedAt' => []];
    }

    // 旧データ形式（usage/deletedAt キーなし、日付が直接並んでいる）を新形式に移行
    if (!array_key_exists('usage', $store[$token])) {
        $store[$token] = ['usage' => $store[$token], 'deletedAt' => []];
    }

    // deletedHours 形式の古いデータがある場合、deletedAt に移行する
    if (isset($store[$token]['deletedHours'])) {
        unset($store[$token]['deletedHours']);
    }

    // hour 単位 (deletedAt, value) マージ: deletedAt は max、value は新しい deletedAt 側のみ採用候補にして max。
    $clientUsage     = $req['usage'] ?? [];
    $clientDeletedAt = $req['deletedAt'] ?? [];

    $allDates = array_unique(array_merge(
        array_keys($store[$token]['usage']),
        array_keys($store[$token]['deletedAt']),
        is_array($clientUsage) ? array_keys($clientUsage) : [],
        is_array($clientDeletedAt) ? array_keys($clientDeletedAt) : []
    ));

    foreach ($allDates as $date) {
        $serverHours    = $store[$token]['usage'][$date]     ?? array_fill(0, 24, 0);
        $serverDeleted  = $store[$token]['deletedAt'][$date] ?? [];
        $clientHours    = $clientUsage[$date]     ?? array_fill(0, 24, 0);
        $clientDeleted  = $clientDeletedAt[$date] ?? [];

        if (!is_array($serverHours)   || count($serverHours)   !== 24) $serverHours   = array_fill(0, 24, 0);
        if (!is_array($clientHours)   || count($clientHours)   !== 24) $clientHours   = array_fill(0, 24, 0);
        if (!is_array($serverDeleted)) $serverDeleted = [];
        if (!is_array($clientDeleted)) $clientDeleted = [];

        $newHours   = array_fill(0, 24, 0);
        $newDeleted = [];

        for ($h = 0; $h < 24; $h++) {
            $sd = isset($serverDeleted[$h]) ? (int)$serverDeleted[$h] : 0;
            $cd = isset($clientDeleted[$h]) ? (int)$clientDeleted[$h] : 0;
            $md = max($sd, $cd);
            if ($md > 0) $newDeleted[$h] = $md;

            $sv = (int)($serverHours[$h] ?? 0);
            $cv = (int)($clientHours[$h] ?? 0);
            $val = 0;
            if ($sd >= $cd) $val = max($val, $sv);
            if ($cd >= $sd) $val = max($val, $cv);
            $newHours[$h] = $val;
        }

        // すべての値が 0 かつ削除マークも無ければ日付ごと除外
        $hasUsage = false;
        foreach ($newHours as $v) { if ($v > 0) { $hasUsage = true; break; } }
        if ($hasUsage || count($newDeleted) > 0) {
            $store[$token]['usage'][$date] = $newHours;
            if (count($newDeleted) > 0) {
                $store[$token]['deletedAt'][$date] = $newDeleted;
            } else {
                unset($store[$token]['deletedAt'][$date]);
            }
        } else {
            unset($store[$token]['usage'][$date]);
            unset($store[$token]['deletedAt'][$date]);
        }
    }

    file_put_contents($DATA_FILE, json_encode($store));

    // deletedAt を返すことでクライアントが他端末の削除情報を受け取れる
    echo json_encode([
        'usage'        => $store[$token]['usage'],
        'deletedAt'    => $store[$token]['deletedAt'],
    ]);
    exit;
}

echo json_encode(['error' => 'not found', 'path' => $path]);
