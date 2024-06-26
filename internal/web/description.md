JSON API

本 API 实际的请求格式为 JSON RPC 2.0.

OpenAPI 定义的 `operationId` 为 json rpc 的请求方法，
`Request body` 为 json rpc 响应的 `params`。
`Response body` 为 json rpc 响应的 `result`。

方法也可能会返回 error ，但是 openapi 中没有完整定义。
