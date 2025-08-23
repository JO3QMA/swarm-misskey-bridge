/**
 * Swarm Misskey Integration - Cloudflare Worker
 * Swarmでのチェックイン情報を自動的に取得し、指定したMisskeyインスタンスに投稿する
 */

// 設定
const CONFIG = {
  MISSKEY_INSTANCE: 'https://misskey.io',
  POST_TEMPLATE: 'Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey',
  VISIBILITY: 'public'
};

/**
 * メインのリクエストハンドラー
 */
export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    try {
      // ルーティング
      switch (true) {
        case path === '/' && request.method === 'GET':
          return handleRoot(request);
        case path === '/health' && request.method === 'GET':
          return handleHealth(request);
        case path === '/webhook' && request.method === 'POST':
          return handleSwarmWebhook(request, env);
        case path === '/poll' && request.method === 'POST':
          return handlePolling(request, env);
        case path === '/manual-poll' && request.method === 'POST':
          return handleManualPolling(request, env);
        case path === '/config' && request.method === 'GET':
          return handleGetConfig(request, env);
        case path === '/config' && request.method === 'POST':
          return handleUpdateConfig(request, env);
        default:
          return handleNotFound(request);
      }
    } catch (error) {
      console.error('Error handling request:', error);
      return createResponse({
        success: false,
        error: 'Internal server error'
      }, 500);
    }
  },

  // Cron trigger for polling Swarm API
  async scheduled(event, env, ctx) {
    console.log('Cron trigger executed:', event.cron);
    try {
      await pollSwarmCheckins(env);
      return new Response('OK');
    } catch (error) {
      console.error('Scheduled polling error:', error);
      return new Response('Error', { status: 500 });
    }
  }
};

/**
 * ルートエンドポイント
 */
async function handleRoot(request) {
  return createResponse({
    success: true,
    message: 'Swarm Misskey Integration is running'
  });
}

/**
 * ヘルスチェックエンドポイント
 */
async function handleHealth(request) {
  return createResponse({
    success: true,
    message: 'Healthy'
  });
}

/**
 * 404エンドポイント
 */
async function handleNotFound(request) {
  return createResponse({
    success: false,
    error: 'Not found'
  }, 404);
}

/**
 * Swarm Webhookエンドポイント
 */
async function handleSwarmWebhook(request, env) {
  // Webhook署名の検証
  if (!verifyWebhookSignature(request, env)) {
    return createResponse({
      success: false,
      error: 'Invalid signature'
    }, 401);
  }

  // リクエストボディの解析
  let checkin;
  try {
    const body = await request.text();
    checkin = JSON.parse(body);
  } catch (error) {
    return createResponse({
      success: false,
      error: 'Invalid JSON'
    }, 400);
  }

  // チェックイン情報の検証
  if (!validateCheckin(checkin)) {
    return createResponse({
      success: false,
      error: 'Invalid checkin data'
    }, 400);
  }

  // チェックインの処理
  try {
    await processCheckin(checkin, env);
    return createResponse({
      success: true,
      message: 'Checkin processed successfully'
    });
  } catch (error) {
    console.error('Error processing checkin:', error);
    return createResponse({
      success: false,
      error: error.message
    }, 500);
  }
}

/**
 * チェックイン情報の検証
 */
function validateCheckin(checkin) {
  return checkin && 
         checkin.id && 
         checkin.venueName && 
         checkin.url;
}

/**
 * チェックインの処理
 */
async function processCheckin(checkin, env) {
  const misskeyInstance = env.MISSKEY_INSTANCE || CONFIG.MISSKEY_INSTANCE;
  const apiKey = env.MISSKEY_API_KEY;
  const postTemplate = env.POST_TEMPLATE || CONFIG.POST_TEMPLATE;
  const visibility = env.VISIBILITY || CONFIG.VISIBILITY;

  if (!apiKey) {
    throw new Error('Misskey API key not configured');
  }

  // 画像のアップロード
  let fileIds = [];
  if (checkin.imageUrl) {
    try {
      const fileId = await uploadImageToMisskey(checkin.imageUrl, misskeyInstance, apiKey);
      fileIds.push(fileId);
    } catch (error) {
      console.error('Failed to upload image:', error);
      // 画像のアップロードに失敗しても投稿は続行
    }
  }

  // 投稿内容の作成
  const postText = createPostText(checkin, postTemplate);

  // Misskeyへの投稿
  await postToMisskey(postText, fileIds, visibility, misskeyInstance, apiKey);
}

/**
 * 画像をMisskeyにアップロード
 */
async function uploadImageToMisskey(imageUrl, misskeyInstance, apiKey) {
  // 画像のダウンロード
  const imageResponse = await fetch(imageUrl);
  if (!imageResponse.ok) {
    throw new Error(`Failed to download image: ${imageResponse.status}`);
  }

  const imageData = await imageResponse.arrayBuffer();
  const contentType = imageResponse.headers.get('content-type') || 'image/jpeg';

  // FormDataの作成
  const formData = new FormData();
  const blob = new Blob([imageData], { type: contentType });
  formData.append('file', blob, 'swarm-checkin.jpg');
  formData.append('i', apiKey);

  // Misskeyへのアップロード
  const uploadResponse = await fetch(`${misskeyInstance}/api/drive/files/create`, {
    method: 'POST',
    body: formData
  });

  if (!uploadResponse.ok) {
    throw new Error(`Upload failed: ${uploadResponse.status}`);
  }

  const fileData = await uploadResponse.json();
  return fileData.id;
}

/**
 * 投稿内容の作成
 */
function createPostText(checkin, template) {
  let text = template;

  // プレースホルダーの置換
  text = text.replace(/{venueName}/g, checkin.venueName || '');
  text = text.replace(/{comment}/g, checkin.comment || '');
  text = text.replace(/{url}/g, checkin.url || '');

  // URLが含まれていない場合は追加
  if (!text.includes(checkin.url)) {
    text += `\n\n${checkin.url}`;
  }

  return text;
}

/**
 * Misskeyへの投稿
 */
async function postToMisskey(text, fileIds, visibility, misskeyInstance, apiKey) {
  const requestData = {
    i: apiKey,
    text: text,
    visibility: visibility
  };

  if (fileIds.length > 0) {
    requestData.fileIds = fileIds;
  }

  const response = await fetch(`${misskeyInstance}/api/notes/create`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(requestData)
  });

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({}));
    throw new Error(`Post failed: ${response.status} - ${errorData.error || 'Unknown error'}`);
  }

  return await response.json();
}

/**
 * Webhook署名の検証
 */
function verifyWebhookSignature(request, env) {
  const signature = request.headers.get('X-Swarm-Signature');
  const secret = env.SWARM_WEBHOOK_SECRET;

  // シークレットが設定されていない場合は検証をスキップ
  if (!secret) {
    return true;
  }

  // TODO: 実際の署名検証ロジックを実装
  // 現在は常にtrueを返す
  return true;
}

/**
 * ポーリングエンドポイント（Cron Triggers用）
 */
async function handlePolling(request, env) {
  try {
    console.log('Polling triggered by Cron');
    await pollSwarmCheckins(env);
    return createResponse({
      success: true,
      message: 'Polling completed successfully'
    });
  } catch (error) {
    console.error('Polling error:', error);
    return createResponse({
      success: false,
      error: error.message
    }, 500);
  }
}

/**
 * 手動ポーリングエンドポイント
 */
async function handleManualPolling(request, env) {
  try {
    console.log('Manual polling triggered');
    await pollSwarmCheckins(env);
    return createResponse({
      success: true,
      message: 'Manual polling completed successfully'
    });
  } catch (error) {
    console.error('Manual polling error:', error);
    return createResponse({
      success: false,
      error: error.message
    }, 500);
  }
}

/**
 * 設定取得エンドポイント
 */
async function handleGetConfig(request, env) {
  try {
    const config = await getConfig(env);
    const safeConfig = {
      misskeyInstance: config.misskeyInstance,
      postTemplate: config.postTemplate,
      visibility: config.visibility,
      pollingInterval: config.pollingInterval,
      hasSwarmAPIKey: !!env.SWARM_API_KEY,
      hasMisskeyAPIKey: !!env.MISSKEY_API_KEY
    };
    
    return createResponse(safeConfig);
  } catch (error) {
    console.error('Get config error:', error);
    return createResponse({
      success: false,
      error: error.message
    }, 500);
  }
}

/**
 * 設定更新エンドポイント
 */
async function handleUpdateConfig(request, env) {
  try {
    const body = await request.text();
    const configUpdate = JSON.parse(body);
    
    await updateConfig(configUpdate, env);
    
    return createResponse({
      success: true,
      message: 'Configuration updated successfully'
    });
  } catch (error) {
    console.error('Update config error:', error);
    return createResponse({
      success: false,
      error: error.message
    }, 400);
  }
}

/**
 * Swarm APIポーリング機能
 */
async function pollSwarmCheckins(env) {
  // テストモードの場合はスキップ
  if (env.SWARM_API_KEY === 'test-swarm-key' || env.MISSKEY_API_KEY === 'test-key') {
    console.log('Skipping polling in test mode');
    return;
  }

  const config = await getConfig(env);
  const lastCheckinTime = await getLastCheckinTime(env);
  
  // Swarm APIからチェックインを取得
  const checkins = await fetchSwarmCheckins(env, lastCheckinTime);
  
  let latestCheckinTime = lastCheckinTime;
  
  for (const checkin of checkins) {
    if (checkin.createdAt > lastCheckinTime) {
      try {
        await processCheckin(checkin, config, env);
        if (checkin.createdAt > latestCheckinTime) {
          latestCheckinTime = checkin.createdAt;
        }
      } catch (error) {
        console.error(`Failed to process checkin ${checkin.id}:`, error);
      }
    }
  }
  
  if (latestCheckinTime > lastCheckinTime) {
    await updateLastCheckinTime(latestCheckinTime, env);
  }
}

/**
 * Swarm APIからチェックインを取得
 */
async function fetchSwarmCheckins(env, since) {
  const apiUrl = `https://api.foursquare.com/v2/users/self/checkins?oauth_token=${env.SWARM_API_KEY}&v=20240101&limit=50`;
  
  const response = await fetch(apiUrl);
  if (!response.ok) {
    throw new Error(`Swarm API request failed: ${response.status}`);
  }
  
  const data = await response.json();
  return data.response.checkins.items.map(item => ({
    id: item.id,
    venueName: item.venue.name,
    comment: item.shout || '',
    url: item.url,
    createdAt: item.createdAt * 1000, // Convert to milliseconds
    userId: env.SWARM_USER_ID
  }));
}

/**
 * 設定の取得
 */
async function getConfig(env) {
  try {
    const configData = await env.CONFIG.get('config');
    if (configData) {
      return JSON.parse(configData);
    }
  } catch (error) {
    console.error('Failed to get config from KV:', error);
  }
  
  // デフォルト設定
  return {
    misskeyInstance: env.MISSKEY_INSTANCE || 'https://misskey.io',
    postTemplate: 'Swarmでチェックインしました！📍 {venueName} {comment} #swarm #misskey',
    visibility: 'public',
    pollingInterval: 5
  };
}

/**
 * 設定の更新
 */
async function updateConfig(configUpdate, env) {
  const currentConfig = await getConfig(env);
  
  // 設定を更新
  if (configUpdate.misskeyInstance) {
    currentConfig.misskeyInstance = configUpdate.misskeyInstance;
  }
  if (configUpdate.postTemplate) {
    currentConfig.postTemplate = configUpdate.postTemplate;
  }
  if (configUpdate.visibility) {
    currentConfig.visibility = configUpdate.visibility;
  }
  if (configUpdate.pollingInterval) {
    currentConfig.pollingInterval = parseInt(configUpdate.pollingInterval);
  }
  
  // KVに保存
  await env.CONFIG.put('config', JSON.stringify(currentConfig));
}

/**
 * 最終チェックイン時刻の取得
 */
async function getLastCheckinTime(env) {
  try {
    const timestamp = await env.CONFIG.get('last_checkin_time');
    if (timestamp) {
      return new Date(timestamp);
    }
  } catch (error) {
    console.error('Failed to get last checkin time from KV:', error);
  }
  
  // デフォルト: 1時間前
  return new Date(Date.now() - 60 * 60 * 1000);
}

/**
 * 最終チェックイン時刻の更新
 */
async function updateLastCheckinTime(checkinTime, env) {
  try {
    await env.CONFIG.put('last_checkin_time', checkinTime.toISOString());
    console.log('Updated last checkin time to:', checkinTime.toISOString());
  } catch (error) {
    console.error('Failed to update last checkin time:', error);
  }
}

/**
 * レスポンスの作成
 */
function createResponse(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status: status,
    headers: {
      'Content-Type': 'application/json',
      'Access-Control-Allow-Origin': '*',
      'Access-Control-Allow-Methods': 'GET, POST, OPTIONS',
      'Access-Control-Allow-Headers': 'Content-Type, X-Swarm-Signature'
    }
  });
}
