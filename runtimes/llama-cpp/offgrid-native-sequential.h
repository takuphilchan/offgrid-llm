// OffGrid regression for the pinned upstream native JSON per-call parser.
// Synthetic parser/grammar checks only; no model or executable tool required.
#include "json-schema-to-grammar.h"
#include "../src/llama-grammar.h"
#include "../src/unicode.h"
#include <regex>

static bool offgrid_grammar_accepts(const std::string & grammar_text, const std::string & text,
                                    const std::string & trigger = "") {
    if (!trigger.empty()) {
        // Exercise the runtime's trigger slicing and the grammar independently
        // of model tokenization. Real sampler activation is an installed test.
        const llama_grammar_trigger_pattern pattern{trigger, std::regex(trigger)};
        const size_t start = pattern.find(text);
        return start == std::string::npos || offgrid_grammar_accepts(grammar_text, text.substr(start));
    }
    auto * grammar = llama_grammar_init_impl(nullptr, grammar_text.c_str(), "root", false, nullptr, 0, nullptr, 0);
    if (!grammar) {
        return false;
    }
    bool accepted = false;
    try {
        size_t offset = 0;
        while (offset < text.size()) {
            auto codepoint = unicode_cpt_from_utf8(text, offset);
            llama_grammar_accept_token(*grammar, 0, unicode_cpt_to_utf8(codepoint));
            if (llama_grammar_get_stacks(grammar).empty()) {
                break;
            }
        }
        if (offset == text.size()) {
            for (const auto & stack : llama_grammar_get_stacks(grammar)) {
                accepted |= stack.empty();
            }
        }
    } catch (const std::exception &) {
        accepted = false;
    }
    llama_grammar_free_impl(grammar);
    return accepted;
}

static void test_offgrid_bare_json(testing & t) {
    std::ifstream file("models/templates/meta-llama-Llama-3.1-8B-Instruct.jinja");
    t.assert_true("pinned template available", file.good());
    const std::string source((std::istreambuf_iterator<char>(file)), std::istreambuf_iterator<char>());
    common_chat_template tmpl(source, "", "");
    autoparser::generation_params inputs;
    inputs.messages = json::parse(R"([{"role":"user","content":"hello"}])");
    inputs.tools = json::parse(R"([{"type":"function","function":{"name":"read_fixture","parameters":{"type":"object","properties":{"limit":{"type":"integer","minimum":1,"maximum":2000}},"required":["limit"],"additionalProperties":false}}}])");
    inputs.parallel_tool_calls = false;
    inputs.reasoning_format = COMMON_REASONING_FORMAT_NONE;
    for (bool required : {false, true}) {
        inputs.tool_choice = required ? COMMON_CHAT_TOOL_CHOICE_REQUIRED : COMMON_CHAT_TOOL_CHOICE_AUTO;
        auto data = autoparser::peg_generator::generate_parser(tmpl, inputs);
        t.assert_true("bare JSON native grammar present", !data.grammar.empty());
        t.assert_equal("only optional grammar is lazy", !required, data.grammar_lazy);
        if (data.grammar.empty()) { continue; }
        const std::string call = R"({"name":"read_fixture","parameters":{"limit":2000}})";
        const std::string prefix = required ? data.generation_prompt : "";
        t.assert_true("valid native call accepted", offgrid_grammar_accepts(data.grammar, prefix + call));
        t.assert_true("range enforced", !offgrid_grammar_accepts(data.grammar, prefix + R"({"name":"read_fixture","parameters":{"limit":1000000}})"));
        t.assert_true("no native call batch", !offgrid_grammar_accepts(data.grammar, prefix + call + ";" + call));
        t.assert_true("no unknown tool", !offgrid_grammar_accepts(data.grammar, prefix + R"({"name":"unknown","parameters":{"limit":1}})"));
        if (!required) {
            t.assert_true("bounded native trigger provided", !data.grammar_triggers.empty());
            t.assert_equal("only anchored native trigger", size_t(1), data.grammar_triggers.size());
            if (data.grammar_triggers.empty()) { continue; }
            const std::regex trigger(data.grammar_triggers.front().value);
            const std::string pattern = data.grammar_triggers.front().value;
            t.assert_true("triggered valid call", offgrid_grammar_accepts(data.grammar, call, pattern));
            t.assert_true("triggered leading whitespace", offgrid_grammar_accepts(data.grammar, " \n" + call, pattern));
            t.assert_true("triggered range enforcement", !offgrid_grammar_accepts(data.grammar, R"({"name":"read_fixture","parameters":{"limit":1000000}})", pattern));
            t.assert_true("triggered cardinality enforcement", !offgrid_grammar_accepts(data.grammar, call + ";" + call, pattern));
            t.assert_true("name-first trigger", std::regex_search(call, trigger));
            t.assert_true("whitespace trigger", std::regex_search(std::string(" \n{ \"name\" :"), trigger));
            t.assert_true("wrapper trigger", std::regex_search(std::string(R"({"type":"function", "name":)"), trigger));
            for (const std::string text : {"Hello!", "Example: {\"name\":", "{\"answer\": 42}", "{\"type\":\"other\",\"name\":"}) {
                t.assert_true("ordinary content does not activate native grammar", !std::regex_search(text, trigger));
                t.assert_true("untriggered ordinary content unchanged", offgrid_grammar_accepts(data.grammar, text, pattern));
            }
        }
    }
    inputs.tool_choice = COMMON_CHAT_TOOL_CHOICE_NONE;
    t.assert_true("explicit no-tools unchanged", autoparser::peg_generator::generate_parser(tmpl, inputs).grammar.empty());
}

static void test_offgrid_native_sequential(testing & t) {
    t.test("bare_json_native", test_offgrid_bare_json);
    for (bool parallel : {false, true}) {
        for (bool required : {false, true}) {
            t.test(std::string(parallel ? "parallel" : "sequential") + (required ? "_required" : "_optional"), [&](testing & t) {
                autoparser::generation_params inputs;
                inputs.parallel_tool_calls = parallel;
                inputs.tool_choice = required ? COMMON_CHAT_TOOL_CHOICE_REQUIRED : COMMON_CHAT_TOOL_CHOICE_AUTO;
                inputs.tools = json::parse(R"([{"type":"function","function":{"name":"read_fixture","parameters":{"type":"object","properties":{"path":{"type":"string"}},"required":["path"],"additionalProperties":false}}}])");
                autoparser::analyze_tools tools;
                tools.format.mode = autoparser::tool_format::JSON_NATIVE;
                tools.format.per_call_start = "<tool_call>";
                tools.format.per_call_end = "</tool_call>";
                tools.format.name_field = "name";
                tools.format.args_field = "arguments";
                tools.format.parameter_order = {"name", "arguments"};
                auto parser = build_chat_peg_parser([&](common_chat_peg_builder & p) {
                    autoparser::parser_build_context ctx(p, inputs);
                    return tools.build_parser(ctx);
                });
                const std::string call = R"(<tool_call>{"name":"read_fixture","arguments":{"path":"café 日本語"}}</tool_call>)";
                const std::string grammar = build_grammar([&](const common_grammar_builder & builder) {
                    parser.build_grammar(builder, /* lazy */ true);
                });
                for (const auto & sample : {std::make_pair(call, true), std::make_pair(call + "\n" + call, parallel),
                                           std::make_pair(call.substr(0, call.size() - 3), false)}) {
                    common_peg_parse_context ctx(sample.first);
                    auto parsed = parser.parse(ctx);
                    t.assert_equal("parser cardinality and completeness", sample.second, parsed.success());
                    t.assert_equal("generated grammar cardinality and completeness", sample.second, offgrid_grammar_accepts(grammar, sample.first));
                }
                common_peg_parse_context plain("Hello.");
                t.assert_equal("ordinary text only when optional", !required, parser.parse(plain).success());
            });
        }
    }
}
