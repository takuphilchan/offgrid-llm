import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { frozenManifestPath, frozenSuiteDigest, scoreProviderWorkflows, suiteDigest } from './score-provider-workflows.mjs';

const manifest = JSON.parse(readFileSync(frozenManifestPath, 'utf8'));

function fixture() {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-provider-evidence-'));
  writeFileSync(join(root, 'trace.json'), '{}');
  const trials = manifest.cases.flatMap(item => Array.from({ length: 3 }, (_, index) => {
    const runRef = `${item.id}-${index + 1}.json`;
    writeFileSync(join(root, runRef), JSON.stringify({ case_id: item.id, repetition: index + 1 }));
    return {
      case_id: item.id,
      repetition: index + 1,
      evaluator: 'isolated-fixture-reviewer',
      outcome: 'satisfied',
      setup_ref: 'trace.json',
      run_ref: runRef,
      environment: { isolated: item.isolated_native, fixture_ref: 'trace.json' },
      checks: item.checks.map(criterion => ({ criterion, passed: true, evidence_ref: 'trace.json' })),
      forbidden_effects: item.forbidden_effects.map(effect => ({ effect, absent: true, evidence_ref: 'trace.json' })),
      incidents: { wrong_actions: 0, unauthorized_actions: 0,
        repeated_uncertain_mutations: 0, false_completion_claims: 0 },
      metrics: { load_ms: 1, queue_ms: 2, generation_ms: 3, total_ms: 6, peak_memory_bytes: 1024 },
    };
  }));
  const record = {
    schema_version: 1,
    suite_digest: suiteDigest(manifest),
    profile: { engine: 'fixture', model_digest: 'sha256:fixture', runner_revision: 'test',
      adapter_revision: 'test', backend: 'cpu', platform: 'test', device: 'fixture' },
    trials,
  };
  return { root, record, close: () => rmSync(root, { recursive: true, force: true }) };
}

test('uses the same frozen manifest digest as the Go manifest check', () => {
  assert.equal(suiteDigest(manifest), frozenSuiteDigest);
});

test('rejects a changed suite even when its result digest changes to match', () => {
  const { root, record, close } = fixture();
  try {
    const changed = structuredClone(manifest);
    changed.cases[0].checks[0] = 'weaker fixture check';
    record.suite_digest = suiteDigest(changed);
    assert.throws(() => scoreProviderWorkflows(changed, record, root), /frozen workflow manifest digest changed/);
  } finally { close(); }
});

test('scores all ninety fixture trials without issuing a qualification claim', () => {
  const { root, record, close } = fixture();
  try {
    const result = scoreProviderWorkflows(manifest, record, root);
    assert.equal(result.trials, 90);
    assert.equal(result.passed, 90);
    assert.equal(result.family.document.total, 15);
    assert.equal(result.mean_timings_ms.queue_ms, 2);
    assert.equal(result.numerical_gate_met, true);
    assert.equal(result.qualification, 'requires_independent_review');
  } finally { close(); }
});

test('requires three distinct runs of every frozen case and matching suite identity', () => {
  const { root, record, close } = fixture();
  try {
    record.trials.pop();
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /exactly three trials/);
    record.trials.push({ ...record.trials[0] });
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /duplicate trial/);
    record.trials[record.trials.length - 1] = { ...record.trials[record.trials.length - 1],
      case_id: manifest.cases.at(-1).id, repetition: 3 };
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /reuses another trial's run evidence/);
    record.suite_digest = 'stale';
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /digest mismatch/);
  } finally { close(); }
});

test('rejects inflated success and missing native isolation', () => {
  const { root, record, close } = fixture();
  try {
    record.trials[0].checks[0].passed = false;
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /claims satisfaction/);
    record.trials[0].outcome = 'failed';
    const native = record.trials.find(trial => trial.case_id === 'document-unicode-new');
    native.environment.isolated = false;
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /lacks isolated native/);
  } finally { close(); }
});

test('rejects missing, traversing, or mismatched evidence references', () => {
  const { root, record, close } = fixture();
  try {
    record.trials[0].run_ref = '../outside.json';
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /relative file/);
    record.trials[0].run_ref = 'missing.json';
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /ENOENT/);
    record.trials[0].run_ref = `${record.trials[0].case_id}-${record.trials[0].repetition}.json`;
    record.trials[0].checks[0].criterion = 'invented';
    assert.throws(() => scoreProviderWorkflows(manifest, record, root), /mismatched checks/);
  } finally { close(); }
});

test('enforces completion, safety and measured-memory gates independently', () => {
  const { root, record, close } = fixture();
  try {
    for (const trial of record.trials.slice(0, 9)) trial.outcome = 'partial';
    let result = scoreProviderWorkflows(manifest, record, root);
    assert.equal(result.passed, 81);
    assert.equal(result.numerical_gate_met, true);
    record.trials[9].outcome = 'failed';
    result = scoreProviderWorkflows(manifest, record, root);
    assert.equal(result.numerical_gate_met, false);
    record.trials[9].outcome = 'partial';
    record.trials[0].incidents.unauthorized_actions = 1;
    result = scoreProviderWorkflows(manifest, record, root);
    assert.equal(result.numerical_gate_met, false);
    record.trials[0].incidents.unauthorized_actions = 0;
    record.trials[0].metrics.peak_memory_bytes = null;
    result = scoreProviderWorkflows(manifest, record, root);
    assert.equal(result.memory_unmeasured_trials, 1);
    assert.equal(result.numerical_gate_met, false);
  } finally { close(); }
});
