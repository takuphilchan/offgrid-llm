# Pinned llama.cpp compatibility patch

`native-sequential.patch` fixes per-call JSON repetition in upstream revision
`b95502ba9aa0eb73a2f4fc8878d7fbe6a847a0b9`. Wrapped calls honor the requested
cardinality, and optional calls do not require a nonempty repetition. The native
format, tool schemas, result history and final OffGrid validation are unchanged.

Revision 2 also activates the existing native grammar for optional, name-first
untagged JSON calls. Its anchored trigger recognizes only the analyzed native
envelope at the start of a reply, not JSON examples embedded in prose. This
enforces registered names, argument constraints and sequential cardinality once
the model begins a call; ordinary answers remain unconstrained. Unknown native
shapes retain their existing behavior and independent validation.

The build-time script rejects other commits, dirty patch targets and repeated
application. It never edits an installed runtime. CPU/CUDA Docker builds and
release bundles apply the same patch. The worker reports the distinct commit
suffix `-offgrid-seq2`, so runtime evidence cannot silently carry over from the
unpatched worker. Existing runtime downloads are not automatically replaced.

Apply and test in a separate checkout of the pinned source:

```sh
sh /path/to/offgrid/runtimes/llama-cpp/apply-patches.sh /path/to/llama.cpp
cmake -S /path/to/llama.cpp -B /path/to/llama.cpp/build \
  -DLLAMA_BUILD_TESTS=ON -DGGML_CUDA=OFF -DLLAMA_BUILD_UI=OFF
cmake --build /path/to/llama.cpp/build --parallel 3 --target test-chat-auto-parser
cd /path/to/llama.cpp
build/bin/test-chat-auto-parser
```

Run the whole test binary: its filter applies independently to nested test
names, so filtering only the parent name would skip the new assertions.
The added test checks parser and generated-grammar acceptance for single,
multiple and truncated wrapped calls, optional/required modes and Unicode.
It also exercises the pinned Llama template's bare JSON grammar, range limits,
unknown tools and anchored trigger exclusions.
These model-free tests do not qualify Hermes/OpenClaw workflow quality. Exact
file read-back and greeting checks remain separate release gates; see
[integration evidence](../../docs/quality/evidence/external-integrations-2026-10-22.md).
