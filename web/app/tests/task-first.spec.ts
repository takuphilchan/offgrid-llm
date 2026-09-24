import { expect, test, type Page } from "@playwright/test";
import { taskWorkspaceText } from "../src/i18n/task-workspace";

const id = "run-" + "a".repeat(32),
  model = "task-test-model";

test("task styles preserve the established monochrome layers in both themes", async ({
  page,
}, testInfo) => {
  await fixture(page);
  for (const theme of ["light", "dark"]) {
    await page.goto("/ui/#/agents/new");
    await page.evaluate(
      (theme) => localStorage.setItem("offgrid.theme", theme),
      theme,
    );
    await page.reload();
    await expect(
      page.getByRole("heading", { name: "What would you like done?" }),
    ).toBeVisible();
    const palette = await page.evaluate(() => {
      const style = getComputedStyle(document.documentElement);
      return {
        accent: style.getPropertyValue("--accent").trim(),
        surface: style.getPropertyValue("--surface-1").trim(),
        font: style.fontFamily,
        link: getComputedStyle(document.querySelector(".task-management a")!)
          .color,
        text: style.getPropertyValue("--text").trim(),
      };
    });
    expect(palette.accent).toBe(theme === "light" ? "#171717" : "#f5f5f4");
    expect(palette.surface).toBe(theme === "light" ? "#ffffff" : "#181819");
    expect(palette.font).toContain("IBM Plex Sans");
    expect(palette.link).toBe(
      theme === "light" ? "rgb(23, 23, 23)" : "rgb(245, 245, 244)",
    );
    await page.screenshot({
      path: testInfo.outputPath(`task-${theme}.png`),
      fullPage: true,
    });
  }
});
async function fixture(
  page: Page,
  { desktop = false, lost = false }: { desktop?: boolean; lost?: boolean } = {},
) {
  await page.addInitScript(
    ({ desktop }) => {
      if (!localStorage.getItem("offgrid.locale"))
        localStorage.setItem("offgrid.locale", "en");
      localStorage.setItem("offgrid.onboarding.complete", "true");
      if (desktop)
        Object.assign(window, {
          electron: {
            isDesktop: true,
            platform: "win32",
            getApiUrl: async () => location.origin,
            getVersion: async () => "test",
            getServerStatus: async () => true,
            getSystemTheme: async () => "light",
            onThemeChange: () => () => {},
            discoverComputerApps: async () => ({
              state: "selecting",
              targets: [
                {
                  id: "opaque-target",
                  title: "Untitled - Notepad",
                  driver: "windows-uia",
                },
              ],
            }),
            startComputerApp: async (request: unknown) => {
              (window as any).hostRequest = request;
              return {
                state: "ready",
                target: { id: "host-session", origin: "Untitled - Notepad" },
              };
            },
            getComputerStatus: async () => ({ state: "idle", installed: true }),
            stopComputerAccess: async (request: unknown) => {
              (window as any).scopedStop = request;
              return {state: 'stopped'};
            },
          },
        });
    },
    { desktop },
  );
  let state = "waiting_for_input",
    prompt = "";
  const submissions: any[] = [];
  const commands: {action: string; data: any}[] = [];
  let extra: Record<string, unknown> = {};
  let inputCount = 0;
  const snapshot = () => ({
    run_id: id,
    task_id: id,
    prompt,
    model,
    status: state,
    steps: [],
    output: state === "completed" ? "Verified result from the saved task." : "",
    pending_approval: null,
    resumable: state === 'interrupted',
    ...extra,
    pending_input:
      state === "waiting_for_input"
        ? {
            id: "input-1",
            call_id: "call-1",
            kind: "computer",
            mode: "app",
            target: "Notepad",
          }
        : null,
  });
  await page.route("**/health", (r) =>
    r.fulfill({ json: { status: "healthy" } }),
  );
  await page.route(/\/(?:api\/v2|v1)\//, async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = {};
    if (path === "/api/v2/system")
      data = {
        product: "offgrid",
        api_version: 2,
        version: "test",
        workspace_id: "workspace-test",
        capabilities: ["task-first-agents-v2", "durable-agent-events-v2"],
      };
    else if (path === "/v1/system/config")
      data = { version: "test", require_auth: false };
    else if (path === "/v1/users/me")
      data = { user: null, authenticated: false, guest: true };
    else if (path === "/v1/models")
      data = { data: [{ id: model, type: "chat", context_window: 8192 }] };
    else if (path === "/v1/agents/tasks" || (path === '/api/v2/jobs' && route.request().method() === 'GET'))
      data = prompt
        ? [
            {
              id,
              prompt,
              status: state,
              created_at: "2026-09-24T00:00:00Z",
              deletable: state === "completed",
            },
          ]
        : [];
    else if (path === "/api/v2/jobs" && route.request().method() === "POST") {
      const body = route.request().postDataJSON();
      submissions.push(body);
      prompt = body.prompt;
      if (lost && submissions.length === 1) return route.abort("failed");
      return route.fulfill({ status: 202, json: snapshot() });
    } else if (path === `/api/v2/jobs/${id}/input`) {
      expect(route.request().postDataJSON()).toEqual({
        input_id: "input-1",
        computer_session: "host-session",
      });
      inputCount++;
      state = "completed";
      return route.fulfill({ status: 202, json: snapshot() });
    } else if (path === `/api/v2/jobs/${id}/events`) {
      return route.fulfill({contentType: 'text/event-stream', body: `data: ${JSON.stringify({...snapshot(), type: 'snapshot'})}\n\n`});
    } else if (path.startsWith(`/api/v2/jobs/${id}/`) && route.request().method() === 'POST') {
      const action = path.split('/').pop()!;
      const data = route.request().postDataJSON(); commands.push({action, data});
      if (action === 'pause' || action === 'takeover') state = 'interrupted';
      if (action === 'takeover') extra.computer_session_expired = true;
      if (action === 'resume') state = 'running';
      if (action === 'cancel') state = 'cancelled';
      if (action === 'steer') extra.instructions = [{request_id: data.request_id, text: data.instruction, at: new Date().toISOString()}];
      return route.fulfill({status: 202, json: snapshot()});
    } else if (path === `/api/v2/jobs/${id}/export`) data = {...snapshot(), format: 'offgrid-task-evidence-v1'};
    else if (path === `/api/v2/jobs/${id}`) data = snapshot();
    else if (path === "/api/v2/computer/sessions") data = { sessions: [] };
    else if (path === "/v1/catalog") data = { models: [] };
    else if (path === "/v1/sessions") data = { sessions: [] };
    else if (path === "/api/v2/computer/status")
      data = { available: false, active_sessions: 0, emergency_stop: false };
    return route.fulfill({ json: data });
  });
  return { submissions, commands, setState: (value: string, fields: Record<string, unknown> = {}) => {state = value; extra = fields;}, inputCount: () => inputCount };
}

test('pause, steering, resume and evidence export use one saved job', async ({page}) => {
  const f = await fixture(page);
  await page.goto('/ui/#/agents/new');
  await page.getByRole('textbox', {name:'Task', exact:true}).fill('Prepare an outline.');
  await page.getByRole('button', {name:'Start task',exact:true}).click();
  await expect(page.getByRole('heading', {name:'Access needed'})).toBeVisible();
  f.setState('running'); await page.reload();
  await page.getByRole('button', {name:'Pause',exact:true}).click();
  await page.getByRole('textbox',{name:'Update instruction'}).fill('Use three sections.');
  await page.getByRole('button',{name:'Save instruction'}).click();
  await expect.poll(() => f.commands.length).toBe(2);
  expect(f.commands[1].action).toBe('steer'); expect(f.commands[1].data.request_id).toBeTruthy();
  await page.getByRole('button',{name:'Resume',exact:true}).click();
  await expect(page.getByRole('button',{name:'Pause',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Stop',exact:true}).click();
  await page.getByText('Context usage',{exact:true}).click();
  const downloading=page.waitForEvent('download'); await page.getByRole('button',{name:'Export evidence'}).click();
  expect((await downloading).suggestedFilename()).toBe(`${id}.json`);
  expect(f.commands.map(command=>command.action)).toEqual(['pause','steer','resume','cancel']);
  expect(f.submissions).toHaveLength(1);
});

test('subtasks are linked evidence, not a second task composer', async ({page}) => {
  const f=await fixture(page); const child='run-'+'b'.repeat(32);
  f.setState('waiting_for_children',{children:[{id:child,spec:{key:'compare',goal:'Compare the two findings',tools:[],depends_on:[]}}]});
  await page.goto(`/ui/#/agents/task/${id}`);
  await expect(page.getByText('Waiting for subtasks',{exact:true}).first()).toBeVisible();
  await expect(page.getByRole('link',{name:'Compare the two findings'})).toHaveAttribute('href',`#/agents/task/${child}`);
  await expect(page.getByRole('button',{name:'Pause',exact:true})).toBeVisible();
  expect(f.submissions).toHaveLength(0);
});

test('completed subtasks do not hide paused instruction updates; active delegation does', async ({page}) => {
  const f=await fixture(page);
  const children=[{id:'run-'+'b'.repeat(32),spec:{key:'past',goal:'Completed analysis',tools:[],depends_on:[]}}];
  f.setState('interrupted',{children,can_steer:true});
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.getByRole('textbox',{name:'Update instruction'}).fill('Summarize the completed findings.');
  await page.getByRole('button',{name:'Save instruction'}).click();
  expect(f.commands.at(-1)?.action).toBe('steer');
  expect(f.commands.at(-1)?.data.instruction).toBe('Summarize the completed findings.');
  f.setState('interrupted',{children,can_steer:false});await page.reload();
  await expect(page.getByRole('link',{name:'Completed analysis'})).toBeVisible();
  await expect(page.getByRole('textbox',{name:'Update instruction'})).toHaveCount(0);
});

test('access-panel Stop cannot stop another task when only the launch picker is open', async ({page}) => {
  const f=await fixture(page,{desktop:true});
  await page.goto(`/ui/#/agents/task/${id}`);
  const requests:string[]=[];
  page.on('request',request=>{if(request.method()==='POST')requests.push(new URL(request.url()).pathname)});
  await page.evaluate(()=>{
    (window as any).stops=[];
    window.electron!.listComputerApplications=async()=>({state:'selecting',targets:[{id:'launcher',title:'Notepad',driver:'windows-uia'}]});
    window.electron!.getComputerStatus=async()=>({state:'ready',installed:true,target:{id:'other-session',origin:'Another task'}});
    window.electron!.stopComputerBrowser=async()=>{(window as any).globalStop=true;return{state:'stopped'}};
    window.electron!.stopComputerAccess=async request=>{(window as any).stops.push(request);return{state:'not_owned'}};
  });
  await page.getByRole('button',{name:'Open an application',exact:true}).click();
  await page.locator('.task-access').getByRole('button',{name:'Stop control',exact:true}).click();
  await expect.poll(()=>f.commands.map(c=>c.action)).toEqual(['cancel']);
  expect(await page.evaluate(()=>(window as any).stops)).toEqual([{requestId:'input-1'}]);
  expect(await page.evaluate(()=>(window as any).globalStop)).toBeUndefined();
  expect(requests).not.toContain('/api/v2/computer/stop');
  expect(f.inputCount()).toBe(0);
});

for (const control of ['panel', 'toolbar']) test(`${control} stop during startup rejects late host attachment even when cancellation is offline`, async ({page}) => {
  const f=await fixture(page,{desktop:true});
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.evaluate(()=>{
    window.electron!.startComputerApp=async()=>new Promise(resolve=>{(window as any).completeStart=resolve;});
    window.electron!.stopComputerBrowser=async()=>{throw Error('global stop must not be called')};
  });
  await page.getByRole('button',{name:'Allow access and continue'}).click();
  await page.getByRole('button',{name:'Allow access and continue'}).click();
  await expect.poll(()=>page.evaluate(()=>typeof (window as any).completeStart)).toBe('function');
  await page.route(`**/api/v2/jobs/${id}/cancel`,route=>route.fulfill({status:503,json:{error:{message:'Offline'}}}));
  const revoked:string[]=[];
  await page.route('**/api/v2/computer/sessions/stop',route=>{revoked.push(route.request().postDataJSON().session_id);return route.fulfill({json:{status:'revoked'}})});
  if (control === 'panel') await page.locator('.task-access').getByRole('button',{name:'Stop control',exact:true}).click();
  else await page.getByRole('button',{name:'Stop',exact:true}).click();
  await expect(page.getByRole('alert').first()).toBeVisible();
  await page.evaluate(()=>(window as any).completeStart({state:'ready',target:{id:'late-session',origin:'Notepad'}}));
  await expect.poll(()=>revoked).toEqual(['late-session']);
  expect(f.inputCount()).toBe(0);
});

test('takeover stops only the matching host session even if server stop fails', async ({page}) => {
  const f=await fixture(page,{desktop:true});f.setState('running',{computer_session:'own-session'});
  await page.goto(`/ui/#/agents/task/${id}`);
  await page.evaluate(()=>{
    window.electron!.getComputerStatus=async()=>({state:'ready',installed:true,target:{id:'own-session',origin:'Editor'}});
    window.electron!.stopComputerBrowser=async()=>{(window as any).stopped=true;return{state:'stopped'}};
    window.electron!.stopComputerAccess=async request=>{if ('session' in request && request.session==='own-session')(window as any).stopped=true;return{state:'stopped'}};
  });
  await page.route(`**/api/v2/jobs/${id}/takeover`, route=>route.fulfill({status:503,json:{error:{message:'Service disconnected'}}}));
  await page.getByRole('button',{name:'Take over',exact:true}).click();
  await expect.poll(()=>page.evaluate(()=>(window as any).stopped)).toBe(true);
  expect(f.submissions).toHaveLength(0);
});

test("task first: no setup wall; saved task hands off without a pairing code or duplicate submission", async ({
  page,
}, testInfo) => {
  const f = await fixture(page);
  await page.goto("/ui/#/agents/new");
  await expect(
    page.getByRole("heading", { name: "What would you like done?" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("task-composer.png"),
    fullPage: true,
  });
  await expect(page.getByText("How should OffGrid work?")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Pair browser" })).toHaveCount(
    0,
  );
  await page
    .getByRole("textbox", { name: "Task", exact: true })
    .fill("Read the open Notepad document and summarize it.");
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Access needed" }),
  ).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath("task-access.png"),
    fullPage: true,
  });
  await expect(
    page.getByRole("link", { name: "Continue in desktop" }),
  ).toHaveAttribute("href", `offgrid://computer?task=${id}`);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Access needed" }),
  ).toBeVisible();
  expect(f.submissions).toHaveLength(1);
  await page.getByRole("button", { name: "New task", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "What would you like done?" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Access needed" }),
  ).toHaveCount(0);
});

test("all nine interface locales keep the task-first composer usable, including RTL", async ({
  page,
}) => {
  await fixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/ui/#/agents/new");
  for (const locale of [
    "en",
    "fr",
    "de",
    "es",
    "ar",
    "sw",
    "sn",
    "nd",
    "zu",
  ] as const) {
    await page.evaluate(
      (locale) => localStorage.setItem("offgrid.locale", locale),
      locale,
    );
    await page.reload();
    await expect(
      page.getByRole("heading", { name: taskWorkspaceText(locale).title }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", {
        name: taskWorkspaceText(locale).start,
        exact: true,
      }),
    ).toBeVisible();
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      locale,
    ).toBeTruthy();
    await expect(page.locator("html")).toHaveAttribute(
      "dir",
      locale === "ar" ? "rtl" : "ltr",
    );
  }
});

test("desktop continues the same saved task with host-issued target and local policy", async ({
  page,
}) => {
  const f = await fixture(page, { desktop: true });
  await page.goto("/ui/#/agents/new");
  await page
    .getByRole("textbox", { name: "Task", exact: true })
    .fill("Read Notepad.");
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await page.getByRole("button", { name: "Allow access and continue" }).click();
  await expect(
    page.getByRole("combobox", { name: "Choose an application" }),
  ).toHaveValue("opaque-target");
  await page.getByRole("button", { name: "Allow access and continue" }).click();
  await expect(
    page.getByText("Verified result from the saved task."),
  ).toBeVisible();
  expect(f.submissions).toHaveLength(1);
  expect(f.inputCount()).toBe(1);
  expect(await page.evaluate(() => (window as any).hostRequest)).toEqual({
    workspace: "workspace-test",
    requestId: "input-1",
    target: "opaque-target",
    approvalMode: "scoped_changes",
  });
});

test("task-first native failures do not prescribe browser repair", async ({
  page,
}) => {
  await fixture(page, { desktop: true });
  await page.goto("/ui/#/agents/new");
  await page.evaluate(() => {
    window.electron!.discoverComputerApps = async () => {
      throw new Error("Unexpected native IPC failure");
    };
  });
  await page
    .getByRole("textbox", { name: "Task", exact: true })
    .fill("Read Notepad.");
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await page.getByRole("button", { name: "Allow access and continue" }).click();
  const error = page.locator(".task-access [role=alert]");
  await expect(error).toBeVisible();
  await expect(error).not.toContainText(/browser|Unexpected native IPC/i);
});

test("lost acknowledgment preserves request identity across reload", async ({
  page,
}) => {
  const f = await fixture(page, { lost: true });
  await page.goto("/ui/#/agents/new");
  await page
    .getByRole("textbox", { name: "Task", exact: true })
    .fill("Read Notepad.");
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await expect(page.locator(".task-surface [role=alert]")).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("textbox", { name: "Task", exact: true }),
  ).toHaveValue("Read Notepad.", { timeout: 15000 });
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Access needed" }),
  ).toBeVisible();
  expect(f.submissions).toHaveLength(2);
  expect(f.submissions[0].request_id).toBe(f.submissions[1].request_id);
});

test("task composer fits narrow screens and keeps the saved result out of a new draft", async ({
  page,
}) => {
  await fixture(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/ui/#/agents/new");
  await page
    .getByRole("textbox", { name: "Task", exact: true })
    .fill("日本語 — مرحبا — Zimbabwe");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  await page.getByRole("button", { name: "Start task", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Access needed" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
});
