const checked = ctx.http.request({
    url: "https://policy.example.com/check",
    method: "POST",
    headers: {"content-type": ["application/json"]},
    body: JSON.stringify({input: JSON.parse(ctx.request.body)}),
    timeoutMs: 300
});
if (checked.status !== 200) {
    throw new Error("policy service failed");
}
if (JSON.parse(checked.body).allow !== true) {
    return ctx.reject(403, {error: "content rejected"});
}
