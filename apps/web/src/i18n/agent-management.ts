import { defineMessages } from './catalog';
import type { LocaleCode } from './index';

export const agentManagement = defineMessages<{ tools: string; external: string; connections: string }>("i18n/agent-management:agentManagement", {
  en: { tools: 'Tools & MCP', external: 'External agents', connections: 'Connect a service to add tools your agents can use.' },
  fr: { tools: 'Outils et MCP', external: 'Agents externes', connections: 'Connectez un service pour ajouter des outils à vos agents.' },
  de: { tools: 'Werkzeuge & MCP', external: 'Externe Agenten', connections: 'Verbinden Sie einen Dienst, um Ihren Agenten Werkzeuge hinzuzufügen.' },
  es: { tools: 'Herramientas y MCP', external: 'Agentes externos', connections: 'Conecta un servicio para añadir herramientas a tus agentes.' },
  ar: { tools: 'الأدوات وMCP', external: 'الوكلاء الخارجيون', connections: 'اربط خدمة لإضافة أدوات يمكن لوكلائك استخدامها.' },
  sw: { tools: 'Zana na MCP', external: 'Wakala wa nje', connections: 'Unganisha huduma ili kuongeza zana ambazo wakala wako wanaweza kutumia.' },
  sn: { tools: 'Maturusi neMCP', external: 'Maagent ekunze', connections: 'Batanidza sevhisi kuwedzera maturusi anoshandiswa nemaagent ako.' },
  nd: { tools: 'Amathuluzi leMCP', external: 'Ama-agent angaphandle', connections: 'Xhuma isevisi ukwengeza amathuluzi angasetshenziswa ngama-agent akho.' },
  zu: { tools: 'Amathuluzi neMCP', external: 'Ama-agent angaphandle', connections: 'Xhuma isevisi ukuze wengeze amathuluzi angasetshenziswa ama-agent akho.' },
});
