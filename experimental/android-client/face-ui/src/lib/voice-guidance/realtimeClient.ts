import { RECEIVING_INPUT_KEYS, type ChoiceOverrides } from './receivingInput';

export function microphoneError(error: unknown) {
  const name = error && typeof error === 'object' && 'name' in error ? String(error.name) : '';
  if (name === 'NotAllowedError' || name === 'SecurityError')
    return 'マイクが許可されていません。Androidのアプリ権限とマイクのアクセス設定を確認してください。';
  if (name === 'NotFoundError')
    return '利用できるマイクが見つかりません。端末の接続を確認してください。';
  if (name === 'NotReadableError' || name === 'AbortError')
    return 'マイクを開けませんでした。他の録音・通話アプリを閉じ、会話PoCを開き直してマイクを押してください。';
  return 'マイクを開始できませんでした。会話PoCを開き直して、もう一度お試しください。';
}

type Callbacks = {
  context?: () => {operation:string;fields?:Record<string,{label:string;type:string}>};
  message: (id: string, role: 'user' | 'assistant', text: string, order?: number) => void;
  propose: (raw: unknown) => unknown;
  review: (raw?: unknown) => unknown;
  choices?: (raw: unknown) => unknown;
  operation?: (raw: unknown) => unknown;
  label?: (raw: unknown) => unknown;
  state: (state: 'connecting' | 'listening' | 'speaking' | 'off') => void;
  error: (message: string) => void;
};

/** Created only by an explicit microphone tap. Never retries or resumes capture automatically. */
export function startRealtime(
  values: Record<string, unknown>,
  options: ChoiceOverrides,
  callbacks: Callbacks,
) {
  const abort = new AbortController();
  const pc = new RTCPeerConnection();
  const audio = new Audio();
  audio.autoplay = true;
  let stream: MediaStream | undefined;
  let closed = false;
  let channel: RTCDataChannel;
  let timer: ReturnType<typeof setTimeout>;
  const transcripts = new Map<string, string>();
  const calls = new Set<string>();
  const order = new Map<string, number>();
  let sequence = 0;
  const position = (id: string) => {
    if (!order.has(id)) order.set(id, ++sequence);
    return order.get(id)!;
  };
  const stop = () => {
    if (closed) return;
    closed = true;
    abort.abort();
    clearTimeout(timer);
    stream?.getTracks().forEach((track) => track.stop());
    pc.close();
    audio.pause();
    audio.srcObject = null;
    callbacks.state('off');
  };
  const fail = (message: string) => {
    if (!closed) {
      stop();
      callbacks.error(message);
    }
  };
  const send = (event: unknown) => {
    if (!closed && channel.readyState === 'open') channel.send(JSON.stringify(event));
  };
  callbacks.state('connecting');
  timer = setTimeout(
    () => fail('接続がタイムアウトしました。マイクを押して再接続できます。'),
    30000,
  );
  pc.ontrack = (event) => {
    audio.srcObject = event.streams[0] ?? new MediaStream([event.track]);
    void audio
      .play()
      .catch(() => fail('音声の再生を開始できません。マイクを押して再開してください。'));
  };
  pc.onconnectionstatechange = () => {
    if (['failed', 'disconnected', 'closed'].includes(pc.connectionState) && !closed)
      fail('音声接続が切れました。候補は残っています。マイクを押して再接続できます。');
  };
  channel = pc.createDataChannel('oai-events');
  channel.onopen = () => {
    if (closed) return;
    clearTimeout(timer);
    timer = setTimeout(
      () => fail('10分経過したため音声を停止しました。必要なら再開してください。'),
      600000,
    );
    callbacks.state('listening');
  };
  channel.onclose = () => {
    if (!closed) fail('音声接続が終了しました。マイクから再開できます。');
  };
  channel.onmessage = (event) => {
    if (closed || typeof event.data !== 'string' || event.data.length > 100000) return;
    try {
      const e = JSON.parse(event.data);
      if (e.type === 'error') {
        fail('音声サービスでエラーが発生しました。候補を確認して再開してください。');
        return;
      }
      if (e.type === 'input_audio_buffer.speech_started') {
        if (e.item_id) position(e.item_id);
        callbacks.state('listening');
      }
      if (
        ['conversation.item.added', 'conversation.item.created'].includes(e.type) &&
        e.item?.role === 'user'
      )
        position(e.item.id);
      if (e.type === 'response.created' && e.response?.id) position(e.response.id);
      if (e.type === 'output_audio_buffer.started') callbacks.state('speaking');
      if (e.type === 'output_audio_buffer.stopped') callbacks.state('listening');
      if (
        e.type === 'conversation.item.input_audio_transcription.completed' &&
        typeof e.transcript === 'string'
      )
        callbacks.message(e.item_id, 'user', e.transcript, position(e.item_id));
      if (
        ['response.output_audio_transcript.delta', 'response.output_text.delta'].includes(e.type) &&
        typeof e.delta === 'string'
      ) {
        const id = e.response_id ?? e.item_id;
        const text = ((transcripts.get(id) ?? '') + e.delta).slice(-4000);
        transcripts.set(id, text);
        if (transcripts.size > 32) transcripts.delete(transcripts.keys().next().value!);
        callbacks.message(id, 'assistant', text, position(id));
      }
      if (e.type === 'response.done') {
        if (e.response?.status === 'failed') {
          fail('音声応答を完了できませんでした。接続を再開してください。');
          return;
        }
        let needsReply = false;
        for (const item of e.response?.output ?? []) {
          if (item.type !== 'function_call' || calls.has(item.call_id)) continue;
          calls.add(item.call_id);
          let output: object;
          try {
            const args = JSON.parse(item.arguments || '{}');
            let result: unknown;
            if (item.name === 'propose_fields') result = callbacks.propose(args);
            else if (item.name === 'show_review') result = callbacks.review(args);
            else if (item.name === 'show_choices' && callbacks.choices)
              result = callbacks.choices(args);
            else if (item.name === 'show_operation' && callbacks.operation)
              result = callbacks.operation(args);
            else if (item.name === 'prepare_label' && callbacks.label)
              result = callbacks.label(args);
            else throw new Error('未対応の操作です');
            output = {
              ok: true,
              saved: false,
              ...(result && typeof result === 'object' ? result : {}),
            };
            if ('needsChoice' in output && output.needsChoice) needsReply = true;
          } catch (error) {
            needsReply = true;
            output = {
              ok: false,
              message: error instanceof Error ? error.message : '入力内容を確認してください',
            };
          }
          send({
            type: 'conversation.item.create',
            item: {
              type: 'function_call_output',
              call_id: item.call_id,
              output: JSON.stringify(output),
            },
          });
        }
        if (needsReply)
          send({
            type: 'response.create',
            response: {
              instructions: 'ツール結果の質問を短く一度だけ尋ねる。操作の説明や反映の報告は不要。',
            },
          });
      }
    } catch {
      fail('音声応答を読み取れませんでした。再接続してください。');
    }
  };
  void (async () => {
    try {
      try {
        stream = await navigator.mediaDevices.getUserMedia({
          audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true },
        });
      } catch (error) {
        fail(microphoneError(error));
        return;
      }
      if (closed) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }
      stream.getTracks().forEach((track) => pc.addTrack(track, stream!));
      const offer = await pc.createOffer();
      if (closed) return;
      await pc.setLocalDescription(offer);
      const response = await fetch('/api/voice/realtime', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        signal: abort.signal,
        body: JSON.stringify({
          sdp: offer.sdp,
          options,
          values: Object.fromEntries(
            Object.entries(values)
              .filter(
                ([key, value]) =>
                  RECEIVING_INPUT_KEYS.includes(key as (typeof RECEIVING_INPUT_KEYS)[number]) &&
                  value != null,
              )
              .map(([key, value]) => [key, String(value)]),
          ),
        }),
      });
      if (!response.ok) {
        const error = await response.json().catch(() => ({}));
        throw new Error(error.message || '音声に接続できませんでした。');
      }
      const sdp = await response.text();
      if (!closed) await pc.setRemoteDescription({ type: 'answer', sdp });
    } catch (error) {
      fail(error instanceof Error ? error.message : 'マイクを開始できませんでした。');
    }
  })();
  return {
    stop,
    inform: (values: Record<string, unknown>) =>
      send({
        type: 'conversation.item.create',
        item: {
          type: 'message',
          role: 'user',
          content: [
            {
              type: 'input_text',
              text: `画面で利用者が確認・選択した値（データ）: ${JSON.stringify(values)}。操作の報告は不要。`,
            },
          ],
        },
      }),
  };
}
