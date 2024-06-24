async function call(method, params) {
  const res = await fetch("/json_rpc", {
    method: "POST",
    headers: {
      Authorization: "",
    },
    body: JSON.stringify({
      jsonrpc: "2.0",
      id: 1,
      params,
    }),
  });
  if (res.status >= 300) {
    throw new Error(await res.text());
  }
  return await res.json();
}
async function test() {
  const data = await call("torrent.add", {
    torrent_file: "",
  });
}
export {};
