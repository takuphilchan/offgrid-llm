// Reviewed OpenClaw 2026.10.4 embedded hooks, not a general tool-policy engine.
// No prompt/result mutation, file writes or permission grants occur here.
const adapter = "openclaw-2026.10.4-files-v1";
type Fields = Record<string, unknown>;
type HookAPI = { registrationMode?: string; logger?: {warn: (message:string)=>void}; on: (name: "before_prompt_build" | "before_tool_call" | "after_tool_call" | "agent_end", handler: (event: any, ctx: any) => any) => void };
const invocationKey = Symbol.for("offgrid.verification.invocation.v1");

export function registerVerification(api: HookAPI) {
  // This pinned host executes through scoped discovery registries as well as
  // full registries. Its metadata-only modes cannot register these hooks.
  if (!["full", "discovery", "tool-discovery"].includes(api.registrationMode ?? "")) return;
  // A discarded catalog registry can load a fresh module instance. Retain only
  // invocation credentials in this foreground process, never in config or tool
  // subprocess environments. This is not isolation from other trusted plugins.
  const scope = globalThis as unknown as Record<symbol, Record<string,string> | undefined>;
  const env = scope[invocationKey] ?? Object.fromEntries(["URL", "TOKEN", "RUN", "ADAPTER"].map(key => {
    const value = process.env[`OFFGRID_VERIFY_${key}`] ?? "";
    delete process.env[`OFFGRID_VERIFY_${key}`];
    return [key, value];
  }));
  if (!Object.values(env).some(Boolean)) return;
  scope[invocationKey] = env;
  const valid = env.ADAPTER === adapter && /^http:\/\/127\.0\.0\.1:[1-9][0-9]{0,4}$/.test(env.URL) &&
    /^[0-9a-f]{64}$/.test(env.TOKEN) && /^[0-9a-f]{48}$/.test(env.RUN);
  let session = "", failed = false;
  const blocked = new Set<string>();
  const pending = new Map<string, { tool: string; params: Fields }>();

  async function send(route: string, fields: Fields) {
    if (!valid || failed) throw new Error("verification unavailable");
    const body = JSON.stringify({ version: 1, adapter, run: env.RUN, ...fields });
    if (Buffer.byteLength(body, "utf8") > 8 * 1024 * 1024) throw new Error("event too large");
    const response = await fetch(env.URL + route, { method: "POST", redirect: "error",
      headers: { Authorization: `Bearer ${env.TOKEN}`, "Content-Type": "application/json" },
      body, signal: AbortSignal.timeout(3000) });
    if (!response.ok) throw new Error("verification rejected");
    const reader = response.body?.getReader();
    if (!reader) throw new Error("missing verification response");
    const chunks: Uint8Array[] = []; let length = 0;
    try {
      for (;;) {
        const {done,value} = await reader.read(); if (done) break;
        length += value.length;
        if (length > 4096) throw new Error("response too large");
        chunks.push(value);
      }
    } finally { await reader.cancel(); }
    const text = Buffer.concat(chunks).toString("utf8");
    const value = JSON.parse(text);
    if (typeof value?.allowed !== "boolean") throw new Error("invalid verification response");
    return value;
  }
  const sessionID = (ctx: any) => ctx.sessionId ?? ctx.sessionKey ?? "";
  function identity(event: any, ctx: any) {
    const ids = { session: sessionID(ctx), turn: event.runId ?? ctx.runId ?? "",
      call: event.toolCallId ?? ctx.toolCallId ?? "" };
    if (!session || ids.session !== session || Object.values(ids).some(v => typeof v !== "string" || !v || v.length > 256)) {
      throw new Error("uncorrelated hook");
    }
    return ids;
  }
  const keyOf = (ids: Fields) => JSON.stringify(ids);
  const kind = (name: string) => name === "read" ? "read" : name === "write" ? "write" : undefined;
  api.on("before_prompt_build", async (_event, ctx) => {
    try {
      if (ctx.sandboxed !== false) throw new Error("host file identity is unverified");
      const current = sessionID(ctx);
      if (session) { if (session !== current) throw new Error("foreign session"); return; }
      if (!(await send("/bind", { session: current })).allowed) throw new Error("binding rejected");
      session = current;
    } catch { failed = true; api.logger?.warn("OffGrid verification hooks could not bind a supported local session; this run cannot be verified."); }
  });
  api.on("before_tool_call", async (event, ctx) => {
    const operation = kind(event.toolName);
    if (!operation) return;
    let code = "verification_unavailable";
    let message = "Verification is unavailable. Inspect the contract; do not claim verification succeeded.";
    try {
      const ids = identity(event, ctx), key = keyOf(ids), params = event.params ?? {};
      // Reject ambiguous aliases instead of guessing which field the tool uses.
      const path = params.path ?? params.file_path;
      const content = operation === "write" ? params.content : "";
      if (typeof path !== "string" || typeof content !== "string" ||
          (params.path !== undefined && params.file_path !== undefined && params.path !== params.file_path)) throw new Error("invalid file arguments");
      const decision = await send("/before", { ...ids, kind: operation, path, content });
      if (decision.allowed) { pending.set(key, { tool: event.toolName, params: { ...params } }); return; }
      code = decision.code;
      if (typeof decision.message === "string" && decision.message.length <= 1024) message = decision.message;
      await send("/after", { ...ids, status: "blocked" });
      blocked.add(key);
    } catch { failed = true; }
    return { block: true, blockReason: `OffGrid file contract rejected this operation: ${code}. ${message}` };
  });
  api.on("after_tool_call", async (event, ctx) => {
    if (!kind(event.toolName)) return;
    try {
      const ids = identity(event, ctx), key = keyOf(ids);
      if (blocked.delete(key)) return;
      const proposal = pending.get(key); pending.delete(key);
      if (!proposal || proposal.tool !== event.toolName) throw new Error("unmatched completion");
      const result = event.result;
      let status = "unknown", complete_read = false;
      if (event.error || result?.isError === true) status = "failed";
      else if (result && Array.isArray(result.content)) {
        status = "completed";
        complete_read = event.toolName === "read" && result.details?.kind === "text" &&
          typeof result.details.content === "string" && !result.details.truncation && !result.details.continuation &&
          (proposal.params.offset === undefined || proposal.params.offset === 1) && proposal.params.limit === undefined;
      }
      await send("/after", { ...ids, status, complete_read });
    } catch { failed = true; }
  });
  api.on("agent_end", async (event, ctx) => {
    try {
      if (!session || sessionID(ctx) !== session || event.success !== true || pending.size) throw new Error("incomplete lifecycle");
      await send("/end", { session, status: "completed" });
    } catch { failed = true; }
  });
}
