import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { APIError, api, type AgentTask } from '../api/client';
import { useWorkspace } from './workspace-context';
import { draftKey, writeDraft } from './drafts';
import { useDraftScope } from './draft-scope';
import { useWorkspaceRefresh } from './workspace-refresh';

export type KnownWork = { kind: 'task' | 'chat'; id: string; title: string; status: string; stale?: boolean };
type Actions = { rememberChat: (name: string, status: string) => void; needsRecovery: (name: string) => boolean; forgetChat: (name: string) => void; claimChat: (name: string) => () => void; seedJobs: (jobs: AgentTask[]) => void; openChat: (name: string) => void };
const ActionsContext = createContext<Actions | null>(null);
const StateContext = createContext<{ entries: KnownWork[]; more: boolean; taskIDs: string[] }>({ entries: [], more: false, taskIDs: [] });
export const useKnownWork = () => useContext(StateContext);
export function useWorkActions() { const value = useContext(ActionsContext); if (!value) throw Error('Work coordinator required'); return value; }
export const selectChatEvent = 'offgrid:select-known-chat';
const terminal = (status: string) => ['completed', 'failed', 'cancelled'].includes(status);
const taskView = () => /^#\/agents(?:\/(workspace|new|task\/[^/]+))?$/.test(window.location.hash);

// Read-only references, not a second execution store. No prompts, transcripts,
// permissions or audio are persisted here. Mounted views own their one follower;
// the coordinator checks only bounded metadata when those views are absent.
export function ActiveWorkProvider({ children, taskFirst }: { children: ReactNode; taskFirst: boolean }) {
  const { workspace, admin } = useWorkspace();
  const draftScope = useDraftScope();
  const [state, setState] = useState<{ entries: KnownWork[]; more: boolean; taskIDs: string[] }>({ entries: [], more: false, taskIDs: [] });
  const model = useRef({ chats: new Map<string, KnownWork>(), claims: new Map<string, number>(), jobs: [] as KnownWork[], more: false, taskIDs: [] as string[] });
  const alive = useRef(true);
  const actions = useMemo<Actions>(() => {
    const publish = () => {
      if (!alive.current) return;
      const next = { entries: [...model.current.jobs, ...model.current.chats.values()], more: model.current.more, taskIDs: model.current.taskIDs };
      setState(previous => JSON.stringify(previous) === JSON.stringify(next) ? previous : next);
    };
    return {
      needsRecovery(name) { const chat = model.current.chats.get(name); return !!chat && !terminal(chat.status); },
      rememberChat(name, status) {
        if (!workspace || !alive.current || !name || name.length > 1024) return;
        const chats = model.current.chats;
        if (!chats.has(name) && chats.size >= 16) chats.delete(chats.keys().next().value!);
        chats.set(name, { kind: 'chat', id: name, title: name.slice(0, 180), status }); publish();
      },
      forgetChat(name) { model.current.chats.delete(name); publish(); },
      claimChat(name) {
        if (!name) return () => {};
        const claims = model.current.claims; claims.set(name, (claims.get(name) ?? 0) + 1);
        return () => { const count = (claims.get(name) ?? 1) - 1; if (count) claims.set(name, count); else claims.delete(name); };
      },
      seedJobs(jobs) {
        if (!workspace || !admin || !taskFirst || !alive.current) return;
        const active = jobs.filter(job => !job.parent_id && !terminal(job.status));
        // Read-only link identities from the already-fetched permitted snapshot,
        // not guessed from event IDs or a second network/history enumeration.
        model.current.taskIDs = jobs.slice(0, 200).map(job => job.id);
        model.current.jobs = active.slice(0, 16).map(job => ({ kind: 'task', id: job.id, title: job.prompt.slice(0, 180), status: job.status }));
        model.current.more = active.length > 16; publish();
      },
      openChat(name) {
        if (!workspace || !model.current.chats.has(name)) return;
        writeDraft(draftKey(draftScope, 'active-chat'), name);
        window.location.hash = '#/chat';
        window.dispatchEvent(new CustomEvent(selectChatEvent, { detail: name }));
      },
    };
  }, [workspace, admin, taskFirst, draftScope]);
  const check = useRef<AbortController | null>(null);
  const tick = async () => {
    if (!workspace || check.current || !alive.current) return;
    const controller = new AbortController(); check.current = controller;
    try {
      if (admin && taskFirst && !taskView()) {
        try { const jobs = await api.jobs(controller.signal); if (!controller.signal.aborted) actions.seedJobs(jobs); }
        catch (error) {
          if (controller.signal.aborted || !alive.current) return;
          if (error instanceof APIError && [401, 403, 404].includes(error.status)) actions.seedJobs([]);
          else { model.current.jobs = model.current.jobs.map(job => ({ ...job, stale: true })); setState(current => ({ ...current, entries: [...model.current.jobs, ...model.current.chats.values()] })); }
        }
      }
      const pending = [...model.current.chats.values()].filter(chat => !terminal(chat.status) && !model.current.claims.has(chat.id));
      // Four reads at a time, rotate remaining references without adding workers.
      for (const chat of pending.slice(0, 4)) {
        if (controller.signal.aborted || model.current.claims.has(chat.id)) continue;
        try {
          const { turn } = await api.currentTurn(chat.id, controller.signal);
          if (controller.signal.aborted || model.current.claims.has(chat.id)) continue;
          if (turn) actions.rememberChat(chat.id, turn.status); else actions.rememberChat(chat.id, 'unavailable');
        } catch (error) {
          if (controller.signal.aborted) return;
          if (error instanceof APIError && [401, 403, 404].includes(error.status)) actions.forgetChat(chat.id);
          else actions.rememberChat(chat.id, 'unavailable');
        }
        const current = model.current.chats.get(chat.id);
        if (current) { model.current.chats.delete(chat.id); model.current.chats.set(chat.id, current); }
      }
    } finally { if (check.current === controller) check.current = null; }
  };
  useWorkspaceRefresh(tick);
  useEffect(() => {
    alive.current = true;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => { await tick(); if (!disposed) timer = setTimeout(() => void poll(), 5000); };
    void poll();
    return () => { disposed = true; alive.current = false; clearTimeout(timer); check.current?.abort(); check.current = null; };
  }, [actions]);
  return <ActionsContext.Provider value={actions}><StateContext.Provider value={state}>{children}</StateContext.Provider></ActionsContext.Provider>;
}
