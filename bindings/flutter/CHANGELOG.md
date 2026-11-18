## 未发布（同步上游 0.3.1 / 0.4.0）

- 播放改走 V5 取址接口，请求参数和签名与 7.4.3.32 版客户端一致，修复取播放模型失败。
- 读取演员作品时先取作品页的 tab 列表，再按「剧作」tab 请求；搜索联想关键词参数改为 `q`。
- 新增选项 `signerUrl` 和 `signerToken`（Go 是 `SignerURL`、`SignerToken`），可把请求签名交给 openhongguo-signer 服务；不设时仍用内置签名，服务不可用时返回 `SIGNER_UNAVAILABLE`，不回退。命令行用 `OHG_SIGNER_URL` / `OHG_SIGNER_TOKEN`。

## 0.1.0

- 第一个版本。
