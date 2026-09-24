import { useEffect, useRef, useState } from "react";
import {
  APIError,
  api,
  type AgentRun,
  type ComputerSession,
} from "../../api/client";
import { useI18n } from "../../i18n";
import { taskWorkspaceText } from "../../i18n/task-workspace";
import {
  approvalPolicyText,
  nativeAppError,
  nativeAppFeedback,
  nativeAppText,
} from "../../i18n/native-app";
import { computerExperience } from "../../i18n/computer-experience";
import { stopOwnedComputer } from './TaskControls';
import { draftKey, readDraft, writeDraft } from "../../lib/drafts";

type Target = { id: string; title: string; driver: string };
type Policy = ComputerSession["approval_mode"];

// Access is an interruption of an existing job, not another task builder. Host
// IDs come only from the trusted picker; model-proposed labels are never IDs.
export function TaskAccess({
  run,
  workspace,
  onResolved,
  cancelSignal,
}: {
  run: AgentRun;
  workspace: string;
  onResolved: () => void;
  cancelSignal?: AbortSignal;
}) {
  const { locale, messages } = useI18n(),
    copy = taskWorkspaceText(locale),
    policy = approvalPolicyText(locale),
    native = nativeAppText(locale),
    experience = computerExperience(locale);
  const input = run.pending_input!;
  const connectionKey = draftKey(
    workspace,
    "task-access",
    `${run.run_id}:${input.id}`,
  );
  const local =
    input.mode === "app"
      ? !!window.electron?.discoverComputerApps
      : !!window.electron?.startComputerBrowser;
  const [mode, setMode] = useState<Policy>("scoped_changes");
  const [targets, setTargets] = useState<Target[]>([]),
    [target, setTarget] = useState("");
  const [launch, setLaunch] = useState(false);
  const [session, setSession] = useState<ComputerSession | null>(null);
  const [url, setURL] = useState(input.url ?? "");
  const [network, setNetwork] = useState<"direct" | "trusted-vpn">("direct");
  const [upload, setUpload] = useState<{ id: string; name: string } | null>(
    null,
  );
  const [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [stopping, setStopping] = useState(false);
  const operation = useRef(0), stopPending = useRef(false);
  const locked = useRef(false),
    mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // Recover a host connection after renderer navigation or a lost response.
  useEffect(() => {
    let live = true;
    void api
      .computerSessions()
      .then((data) => {
        if (!live) return;
        const own = data.sessions.find(
          (s) =>
            (s.run_id === run.run_id && s.id !== input.previous_session) ||
            (!s.run_id && s.id === readDraft(connectionKey)),
        );
        if (own) setSession(own);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [run.run_id, connectionKey]);
  const perform = async (work: (current: () => boolean) => Promise<void>) => {
    if (locked.current || stopPending.current) return;
    const generation = ++operation.current;
    const current = () => operation.current === generation && !cancelSignal?.aborted;
    locked.current = true;
    setBusy(true);
    setError("");
    try {
      await work(current);
    } catch (reason) {
      if (current() && mounted.current)
        setError(
          reason instanceof APIError
            ? reason.message
            : nativeAppError(
                locale,
                reason instanceof Error ? reason.message : "",
                input.mode === "app"
                  ? nativeAppFeedback(locale).unavailable
                  : experience.repair,
              ),
        );
    } finally {
      if (operation.current === generation) {
        locked.current = false;
        if (mounted.current) setBusy(false);
      }
    }
  };
  const attach = async (id: string) => {
    await api.resolveJobInput(run.run_id, input.id, id);
    onResolved();
  };
  const allow = () =>
    perform(async (current) => {
      if (!window.electron?.stopComputerAccess) throw Error('computer_upgrade_required');
      if (session) {
        await attach(session.id);
        return;
      }
      // A handoff releases only the old session belonging to this saved task.
      await stopOwnedComputer(input.previous_session);
      if (!current()) return;
      if (input.mode === "app" && !targets.length) {
        const state = await window.electron!.discoverComputerApps!({
          workspace,
          requestId: input.id,
        });
        if (!current() || !mounted.current) return;
        setTargets(state.targets ?? []);
        setLaunch(false);
        setError(state.targets?.length ? "" : native.empty);
        // Even an exact label match remains a visible selection for local consent.
        const matches = (state.targets ?? []).filter((t) =>
          t.title
            .toLocaleLowerCase()
            .includes(input.target.toLocaleLowerCase()),
        );
        setTarget(matches.length === 1 ? matches[0].id : "");
        return;
      }
      if (input.mode === "app" && launch) {
        await window.electron!.launchComputerApplication!({ id: target });
        if (!current()) return;
        const state = await window.electron!.discoverComputerApps!({
          workspace,
          requestId: input.id,
        });
        if (current() && mounted.current) {
          setTargets(state.targets ?? []);
          setTarget("");
          setLaunch(false);
        }
        return;
      }
      const state =
        input.mode === "app"
          ? await window.electron!.startComputerApp!({
              workspace,
              requestId: input.id,
              target,
              approvalMode: mode,
            })
          : await window.electron!.startComputerBrowser!({
              workspace,
              requestId: input.id,
              origin: url.trim(),
              approvalMode: mode,
              networkMode: network,
              ...(upload ? { uploadGrant: upload.id } : {}),
            });
      if (!current()) {
        // Stop may have raced with a successful host reply. Revoke only that
        // returned session, never a subsequently connected computer task.
        if (state.target) await Promise.allSettled([
          api.stopComputerSession(state.target.id),
          stopOwnedComputer(state.target.id),
        ]);
        return;
      }
      if (!state.target)
        throw Error(state.code ?? "computer_session_unavailable");
      writeDraft(connectionKey, state.target.id);
      if (mounted.current)
        setSession({
          id: state.target.id,
          origin: state.target.origin,
          approval_mode: mode,
          remaining_actions: 100,
        });
      // Starting a host must never submit a second task, even after a timeout.
      await attach(state.target.id);
    });
  const stop = async () => {
    if (stopPending.current) return;
    stopPending.current = true;
    ++operation.current; // A late discovery/start reply must not attach access.
    locked.current = true;
    setStopping(true); setBusy(true); setError('');
    const results = await Promise.allSettled([
      stopOwnedComputer(session?.id, input.id),
      session ? api.stopComputerSession(session.id) : Promise.resolve(),
      // Cancel this saved job too, in case its input POST was already accepted.
      api.jobAction(run.run_id, 'cancel'),
    ]);
    stopPending.current = false; locked.current = false;
    if (!mounted.current) return;
    setStopping(false); setBusy(false);
    if (results.some(result => result.status === 'rejected')) {
      setError(experience.stopUnconfirmed);
      return;
    }
    writeDraft(connectionKey, '');
    setSession(null); setTargets([]); setTarget('');
    onResolved();
  };
  return (
    <section className="task-access" aria-labelledby="task-access-title">
      <h3 id="task-access-title">{copy.needsAccess}</h3>
      <p>{copy.accessHint}</p>
      <strong className="task-access-target">
        {session?.origin ?? input.target}
      </strong>
      {!local ? (
        <>
          <p>{copy.handoff}</p>
          <a
            className="primary-button"
            href={`offgrid://computer?task=${encodeURIComponent(run.run_id)}`}
          >
            {copy.desktop}
          </a>
        </>
      ) : (
        <>
          {!session && (
            <>
              {input.mode === "browser" && (
                <label className="field">
                  <span>{experience.website}</span>
                  <input
                    type="url"
                    value={url}
                    onChange={(e) => setURL(e.target.value)}
                    disabled={busy}
                  />
                </label>
              )}
              <label className="field">
                <span>{policy.title}</span>
                <select
                  value={mode}
                  disabled={busy}
                  onChange={(e) => setMode(e.target.value as Policy)}
                >
                  <option value="ask_every_time">{policy.ask}</option>
                  <option value="scoped_changes">{policy.scoped}</option>
                  <option value="full_task">{policy.full}</option>
                </select>
                <small>
                  {mode === "full_task"
                    ? policy.fullHelp
                    : mode === "scoped_changes"
                      ? policy.scopedHelp
                      : policy.askHelp}
                </small>
              </label>
              {targets.length > 0 && (
                <label className="field">
                  <span>{copy.choose}</span>
                  <select
                    value={target}
                    disabled={busy}
                    onChange={(e) => setTarget(e.target.value)}
                  >
                    <option value="">{copy.chooseHint}</option>
                    {targets.map((t) => (
                      <option key={t.id} value={t.id}>
                        {t.title}
                      </option>
                    ))}
                  </select>
                </label>
              )}
              {input.mode === "browser" && (
                <details>
                  <summary>{experience.networkSettings}</summary>
                  <label className="field">
                    <span>{experience.networkSettings}</span>
                    <select
                      value={network}
                      disabled={busy}
                      onChange={(e) =>
                        setNetwork(e.target.value as typeof network)
                      }
                    >
                      <option value="direct">{experience.directNetwork}</option>
                      <option value="trusted-vpn">
                        {experience.trustedNetwork}
                      </option>
                    </select>
                  </label>
                  {network === "trusted-vpn" && (
                    <p>{experience.networkWarning}</p>
                  )}
                  {window.electron?.selectComputerUpload && (
                    <button
                      className="secondary-button"
                      disabled={busy}
                      onClick={() =>
                        void perform(async () =>
                          setUpload(
                            await window.electron!.selectComputerUpload!(),
                          ),
                        )
                      }
                    >
                      {native.chooseUpload}
                    </button>
                  )}
                  {upload && (
                    <p>
                      {upload.name}{" "}
                      <button
                        className="text-button"
                        disabled={busy}
                        onClick={() => setUpload(null)}
                      >
                        {native.removeUpload}
                      </button>
                    </p>
                  )}
                </details>
              )}
            </>
          )}
          <div className="button-row">
            <button
              className="primary-button"
              disabled={
                busy ||
                (!session &&
                  ((targets.length > 0 && !target) ||
                    (input.mode === "browser" && !url.trim())))
              }
              onClick={() => void allow()}
            >
              {busy
                ? messages.common.loading
                : launch
                  ? native.launch
                  : copy.allow}
            </button>
            {input.mode === "app" &&
              !session &&
              window.electron?.listComputerApplications && (
                <button
                  className="secondary-button"
                  disabled={busy}
                  onClick={() =>
                    void perform(async (current) => {
                      const state =
                        await window.electron!.listComputerApplications!();
                      if (!current()) return;
                      setTargets(state.targets ?? []);
                      setTarget("");
                      setLaunch(true);
                    })
                  }
                >
                  {native.open}
                </button>
              )}
            {(session || targets.length > 0 || busy || error) && (
              <button
                className="secondary-button"
                disabled={stopping}
                onClick={() => void stop()}
              >
                {nativeAppFeedback(locale).stop}
              </button>
            )}
          </div>
        </>
      )}
      {error && <p role="alert">{error}</p>}
    </section>
  );
}
