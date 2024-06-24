import * as types from './types';

type schemas = types.components['schemas']

async function call(method: 'torrent.add', params: schemas['WebAddTorrentRequest']): Promise<schemas['WebAddTorrentResponse']> ;

async function call<T extends keyof types.paths>(method: T, params: any): Promise<any> {
  const id = crypto.randomUUID();

  const res = await fetch(
    '/json_rpc',
    {
      method: 'POST',
      headers: {
        'Authorization': '',
      },
      body: JSON.stringify({
        jsonrpc: '2.0',
        id: id,
        method,
        params
      }),
    }
  );

  if (res.status >= 300) {
    throw new Error(await res.text());
  }

  let data: RpcResposne = await res.json();

  if (typeof data.error !== 'undefined') {
    throw new RpcError(data.error.code, data.error.message);
  }

  return data.result;
}

class RpcError extends Error {
  code: number;

  constructor(code: number, message: string) {
    super(message);
    this.code = code;
  }
}

type RpcResposne = {
  jsonrpc: '2.0';
  id: string,
  result: any
  error: undefined | {
    code: number,
    message: string
  }
}

async function test() {
  const data = await call('torrent.add', {
    torrent_file: ''
  });
}
