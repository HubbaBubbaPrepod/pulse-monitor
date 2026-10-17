import type { ModuleField, ModuleManifest } from './modules';
export type ExtensionTrigger = {
  type: 'manual' | 'interval' | 'event';
  action?: string;
  label?: string;
  mutating?: boolean;
  inputFields?: ModuleField[];
  inputKey?: string;
  resultView?: 'logs' | 'cards' | 'table';
  seconds?: number;
  events?: string[];
  when?: { key: string; values: string[] };
};
export type ExtensionView = ModuleManifest['views'][number] & {
  rowKey?: string;
  actions?: string[];
};
export type ExtensionComponent = {
  id: string;
  location: 'agent' | 'monitor';
  protocol: 2;
  requires: string[];
  fields: ModuleField[];
  triggers: ExtensionTrigger[];
  views?: ExtensionView[];
  binaries: Record<string, { sha256: string }>;
};
export type ExtensionPackage = {
  schemaVersion: 2;
  protocol: 2;
  id: string;
  version: string;
  name: string;
  author: string;
  description: string;
  permissions: string[];
  components: ExtensionComponent[];
};
export type InstalledModule = ModuleManifest | ExtensionPackage;
export type ExtensionConfiguration = {
  enabled: boolean;
  version?: string;
  config: Record<string, string | number | boolean>;
  grants: string[];
  secrets?: string[];
  clearSecrets?: string[];
};
export type ExtensionOperation = {
  id: string;
  moduleId: string;
  componentId: string;
  serverId?: string;
  action: string;
  status: string;
  data?: Record<string, unknown>;
  error?: string;
};
export function componentEndpoint(
  module: string,
  component: string,
  server: string,
  resource: string,
) {
  return `/extensions/${encodeURIComponent(module)}/components/${encodeURIComponent(component)}/${resource}${server ? `?server=${encodeURIComponent(server)}` : ''}`;
}
export function operationMatches(
  actual: Partial<ExtensionOperation>,
  expected: Pick<ExtensionOperation, 'id' | 'moduleId' | 'componentId' | 'serverId' | 'action'>,
) {
  return (
    actual.id === expected.id &&
    actual.moduleId === expected.moduleId &&
    actual.componentId === expected.componentId &&
    (actual.serverId ?? '') === (expected.serverId ?? '') &&
    actual.action === expected.action
  );
}
export function actionAvailable(
  action: ExtensionTrigger,
  row: Record<string, unknown>,
  rowKey: string,
  enabled: boolean,
  grants: string[],
  pending: boolean,
) {
  return (
    enabled &&
    !pending &&
    typeof row[rowKey] === 'string' &&
    String(row[rowKey]).length > 0 &&
    (!action.mutating || grants.includes('actions.execute')) &&
    (!action.when || action.when.values.includes(String(row[action.when.key])))
  );
}
