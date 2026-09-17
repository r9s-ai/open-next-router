const body = JSON.parse(ctx.request.body);
if (body.mapped !== "yes") {
    throw new Error("after_req_map JSON operations must run first");
}
if (ctx.state.before !== 1) {
    throw new Error("request stages must share attempt state");
}
ctx.request.headers["x-policy"] = ["checked", "after-map"];
body.checked = true;
ctx.request.body = JSON.stringify(body);
