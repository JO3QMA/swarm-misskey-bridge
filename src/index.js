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

  // Cron trigger for polling Swarm API (optional)
  async scheduled(event, env, ctx) {
    console.log('Cron trigger executed:', event.cron);
    // TODO: Implement Swarm API polling
    return new Response('OK');
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
