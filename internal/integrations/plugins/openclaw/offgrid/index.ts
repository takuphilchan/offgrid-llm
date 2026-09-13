import { definePluginEntry } from "openclaw/plugin-sdk/plugin-entry";
import {
  configureOpenAICompatibleSelfHostedProviderNonInteractive,
  discoverOpenAICompatibleSelfHostedProvider,
  promptAndConfigureOpenAICompatibleSelfHostedProviderAuth,
} from "openclaw/plugin-sdk/provider-setup";

const id = "offgrid";
const label = "OffGrid";
const defaultBaseUrl = "http://127.0.0.1:11611/v1";
const apiKeyEnvVar = "OFFGRID_API_KEY";
const modelPlaceholder = "phi-3.5-mini-instruct.Q4_K_M";

type OffGridModel = {
  id?: string;
  type?: string;
  capabilities?: string[];
  context_window?: number;
};

// OffGrid's /v1/models includes embeddings as well as chat models. An agent
// catalog must never advertise an embedding-only model for text inference.
async function discoverChatModels(baseUrl: string, apiKey?: string) {
  const response = await fetch(`${baseUrl}/models`, {
    headers: apiKey ? { Authorization: `Bearer ${apiKey}` } : {},
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok) throw new Error(`OffGrid model discovery returned HTTP ${response.status}`);
  const payload = (await response.json()) as { data?: OffGridModel[] };
  if (!Array.isArray(payload.data)) throw new Error("OffGrid model discovery returned an invalid catalog");
  return payload.data.flatMap((model) => {
    if (!model.id || model.type === "embedding" || !model.capabilities?.includes("chat")) return [];
    const contextWindow = Number.isInteger(model.context_window) && model.context_window! > 0
      ? model.context_window!
      : 8192;
    return [{
      id: model.id,
      name: model.id,
      reasoning: false,
      input: ["text"],
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
      contextWindow,
      maxTokens: Math.min(4096, contextWindow - 1),
    }];
  });
}

export default definePluginEntry({
  id,
  name: "OffGrid Provider",
  description: "Private local model inference through OffGrid",
  register(api) {
    api.registerProvider({
      id,
      label,
      docsPath: "/providers/offgrid",
      envVars: [apiKeyEnvVar],
      auth: [{
        id: "custom",
        label,
        hint: "Private local inference through an OffGrid runtime",
        kind: "custom",
        run: (ctx) => promptAndConfigureOpenAICompatibleSelfHostedProviderAuth({
          cfg: ctx.config,
          prompter: ctx.prompter,
          providerId: id,
          providerLabel: label,
          defaultBaseUrl,
          defaultApiKeyEnvVar: apiKeyEnvVar,
          modelPlaceholder,
        }),
        runNonInteractive: (ctx) => configureOpenAICompatibleSelfHostedProviderNonInteractive({
          ctx,
          providerId: id,
          providerLabel: label,
          defaultBaseUrl,
          defaultApiKeyEnvVar: apiKeyEnvVar,
          modelPlaceholder,
        }),
      }],
      catalog: {
        order: "late",
        run: (ctx) => discoverOpenAICompatibleSelfHostedProvider({
          ctx,
          providerId: id,
          buildProvider: async (params) => {
            const baseUrl = (params?.baseUrl?.trim() || defaultBaseUrl).replace(/\/+$/, "");
            return {
              baseUrl,
              api: "openai-completions",
              models: await discoverChatModels(baseUrl, params?.apiKey),
            };
          },
        }),
      },
      wizard: {
        setup: {
          choiceId: id,
          choiceLabel: label,
          choiceHint: "Private local inference through an OffGrid runtime",
          groupId: id,
          groupLabel: label,
          groupHint: "Local and offline-capable",
          methodId: "custom",
        },
        modelPicker: {
          label: "OffGrid (custom)",
          hint: "Enter OffGrid URL + API key + model",
          methodId: "custom",
        },
      },
    });
  },
});
