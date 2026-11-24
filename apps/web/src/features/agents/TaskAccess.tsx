import { useEffect, useId, useRef, useState } from "react";
import {
  APIError,
  api,
  type AgentRun,
  type ComputerSession,
} from "../../api/client";
import { useI18n } from "../../i18n";
import { taskWorkspaceText } from "../../i18n/task-workspace";
import { operationFeedback } from "../../i18n/operation-feedback";
import { taskStopText } from '../../i18n/task-presentation';
import {
  approvalPolicyText,
  nativeAppError,
  nativeAppFeedback,
  nativeAppText,
} from "../../i18n/native-app";
import { computerExperience } from "../../i18n/computer-experience";
import { stopOwnedComputer } from './TaskControls';
import { draftKey, readDraft, writeDraft, useDraft } from "../../lib/drafts";
import { taskAccessText } from '../../i18n/task-access';
import { TaskDesktopAccess } from './TaskDesktopAccess';
import { TaskWorkspaceAccess } from './TaskWorkspaceAccess';
import { additionalBrowserOrigins } from './browser-scope';
import {useBrowserHandoff} from './browser-handoff';
import {browserManualText} from '../../i18n/browser-manual';

type Target = { id: string; title: string; driver: string };
type Policy = ComputerSession["approval_mode"];

// Access is an interruption of an existing job, not another task builder. Host
// IDs come only from the trusted picker; model-proposed labels are never IDs.
export function TaskAccess(props:{run:AgentRun;workspace:string;onResolved:()=>void;cancelSignal?:AbortSignal;initialConsentPending?:boolean}) {
  if(props.run.pending_input?.kind==='worker_question'||props.run.pending_input?.kind==='worker_clarification')return <p role="status">{props.run.pending_input.target}</p>;
  if(props.run.pending_input?.kind==='workspace')return <TaskWorkspaceAccess {...props}/>;
  if(props.run.pending_input?.mode==='desktop'||(props.run.pending_input?.mode==='app'&&props.run.requested_computer_approval_mode&&props.run.requested_computer_scope!=='selected_window'&&window.electron?.desktopComputerProtocol===3))return <TaskDesktopAccess {...props}/>;
  return <WindowTaskAccess {...props}/>;
}
function WindowTaskAccess({
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
  const handoff=useBrowserHandoff(input.mode==='browser'?input.previous_session:undefined),manualCopy=browserManualText(locale);
  // This is an intent to check the exact host operation, never an access grant.
  // Keep it through navigation/lost replies until attachment or explicit Stop
  // is confirmed; the host revalidates the saved task, actor and previous ID.
  const resumeIntent=useDraft(workspace,'browser-resume',`${run.run_id}:${input.id}:${input.previous_session??''}`);
  const resumeRequested=input.mode==='browser'&&!!input.previous_session&&resumeIntent.value===input.previous_session;
  const setResumeRequested=(value:boolean)=>resumeIntent.setValue(value?input.previous_session??'':'');
  const resumeRetained=resumeRequested||handoff.state==='manual'||handoff.state==='resuming';
  const handoffPending=['checking','taking_over','unavailable'].includes(handoff.state);
  const connectionKey = draftKey(
    workspace,
    "task-access",
    `${run.run_id}:${input.id}`,
  );
  const local =
    input.mode === "app"
      ? !!window.electron?.discoverComputerApps
      : !!window.electron?.startComputerBrowser;
  const [legacyMode, setMode] = useState<Policy>("scoped_changes");
  const mode = run.requested_computer_approval_mode ?? legacyMode;
  const [targets, setTargets] = useState<Target[]>([]),
    [target, setTarget] = useState("");
  const [launch, setLaunch] = useState(false);
  const [session, setSession] = useState<ComputerSession | null>(null);
  const [url, setURL] = useState(input.url ?? "");
  const [additionalSites, setAdditionalSites] = useState('');
  const sitesHelpID = useId();
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
      // Electron preserves the main-process error code inside its IPC wrapper.
      // Strip only this host method's wrapper for local presentation.
      const code = reason instanceof Error ? reason.message.replace(/^Error invoking remote method 'computer-(?:start|browser-resume)': Error: /,'') : '';
      if(current()&&mounted.current&&code==='consent_declined')setResumeRequested(false);
      const browserFailure = input.mode === 'browser' ? ({
        network_blocked: experience.networkBlocked,
        network_unavailable: experience.networkUnavailable,
        consent_declined: experience.declined,
        stop_unconfirmed: experience.stopUnconfirmed,
        computer_browser_scope_invalid: experience.invalidScope,
        computer_browser_scope_changed: experience.scopeChanged,
        computer_browser_workspace_unsupported: experience.scopeUnsupported,
        computer_handoff_unconfirmed: manualCopy.unconfirmed,
        computer_resume_unconfirmed: manualCopy.unconfirmed,
        computer_browser_manual_unsupported: experience.scopeUnsupported,
      } as Record<string, string>)[code] : undefined;
      if (current() && mounted.current)
        setError(
          reason instanceof APIError
            ? reason.message
            : browserFailure ?? nativeAppError(
                locale,
                code,
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
    if(input.mode==='browser')setResumeRequested(false);
    onResolved();
  };
  const allow = () =>
    perform(async (current) => {
      if (!window.electron?.stopComputerAccess) throw Error('computer_upgrade_required');
      if (session) {
        await attach(session.id);
        return;
      }
      const additionalOrigins = input.mode === 'browser'&&!resumeRetained ? additionalBrowserOrigins(additionalSites,url.trim()) : [];
      // A handoff releases only the old session belonging to this saved task.
      // A retained browser is consumed by renewed host consent instead.
      if(!resumeRetained)await stopOwnedComputer(input.previous_session);
      if (!current()) return;
      if (input.mode === "app" && !launch && !targets.length) {
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
      if (input.mode === "app" && !target) return;
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
        resumeRetained
          ? await (async()=>{
              if(!input.previous_session||!window.electron?.resumeComputerBrowser)throw Error('computer_browser_manual_unsupported');
              setResumeRequested(true);
              return window.electron.resumeComputerBrowser({workspace,runId:run.run_id,requestId:input.id,session:input.previous_session});
            })()
          : input.mode === "app"
          ? await window.electron!.startComputerApp!({
              workspace,
              requestId: input.id,
              target,
              approvalMode: mode,
            })
          : await window.electron!.startComputerBrowser!({
              workspace,
              requestId: input.id,
              ...(window.electron!.browserTaskBindingProtocol===1?{runId:run.run_id}:{}),
              origin: url.trim(),
              ...(additionalOrigins.length ? {additionalOrigins} : {}),
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
      if(resumeRetained&&state.state==='not_owned'&&mounted.current)setResumeRequested(false);
      if (state.state!=='ready'||!state.target||(resumeRetained&&state.target.id===input.previous_session))
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
      stopOwnedComputer(session?.id??input.previous_session, input.id),
      session ? api.stopComputerSession(session.id) : Promise.resolve(),
      // Cancel this saved job too, in case its input POST was already accepted.
      api.jobAction(run.run_id, 'cancel'),
    ]);
    stopPending.current = false; locked.current = false;
    if (!mounted.current) return;
    setStopping(false); setBusy(false);
    if (results.some(result => result.status === 'rejected')) {
      setError(taskStopText(locale).unconfirmed);
      return;
    }
    writeDraft(connectionKey, '');
    if(input.mode==='browser')setResumeRequested(false);
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
          {!session&&resumeRetained&&<div role="status"><p>{handoff.state==='manual'?manualCopy.manual:manualCopy.pending} {manualCopy.scope}</p><p>{manualCopy.hint}</p><p dir="ltr">{handoff.origin}</p></div>}
          {!session&&handoffPending&&<p role="status">{handoff.state==='unavailable'?manualCopy.unconfirmed:manualCopy.pending}</p>}
          {!session && !resumeRetained && (
            <>
              {input.mode === "browser" && (
                <><label className="field">
                  <span>{experience.website}</span>
                  <input
                    type="url"
                    value={url}
                    onChange={(e) => setURL(e.target.value)}
                    disabled={busy}
                  />
                </label>
                <label className="field">
                  <span id={`${sitesHelpID}-label`}>{experience.additionalSites}</span>
                  <textarea value={additionalSites} onChange={e=>setAdditionalSites(e.target.value)} disabled={busy} rows={3} maxLength={32768} spellCheck={false} dir="ltr" aria-labelledby={`${sitesHelpID}-label`} aria-describedby={sitesHelpID} />
                  <small id={sitesHelpID}>{experience.additionalSitesHint}</small>
                </label></>
              )}
              {run.requested_computer_approval_mode ? <div className="field">
                <span>{taskAccessText(locale).selected}</span>
                <strong>{mode === 'full_task' ? policy.full : mode === 'scoped_changes' ? policy.scoped : policy.ask}</strong>
                <small>{taskAccessText(locale).consent}</small>
              </div> : <label className="field">
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
              </label>}
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
                busy || (!session&&handoffPending) ||
                (!session &&
                  (((launch || targets.length > 0) && !target) ||
                    (input.mode === "browser" && !resumeRetained && !url.trim())))
              }
              onClick={() => void allow()}
            >
              {busy
                ? messages.common.loading
                : !session&&resumeRetained
                  ? resumeRequested||handoff.state==='resuming'?manualCopy.check:manualCopy.resume
                : !session && launch
                  ? native.launch
                  : !session && input.mode === "app" && !targets.length
                    ? copy.choose
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
                      setError(state.targets?.length ? "" : operationFeedback(locale).emptyApplications);
                    })
                  }
                >
                  {native.open}
                </button>
              )}
            {(session || input.previous_session || targets.length > 0 || busy || error) && (
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
