import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { execFileSync } from 'node:child_process';
import Ajv from 'ajv';
import { parse } from 'yaml';
import { DEFAULT_GROUP_ID, GROUP_ALIASES, REGISTERED_GROUPS, resolveGroupId, type GroupConfig } from './group.ts';

const root = resolve(import.meta.dirname, '../../../..');
const json = (path: string) => JSON.parse(readFileSync(resolve(root, path), 'utf8'));
const manifest = json('docs/architecture.manifest.json');
const schema = json('schema/architecture.schema.json');
const registry = json('data/groups.json');
const ingress = parse(readFileSync(resolve(root, 'k8s/manifests/ingress.yaml'), 'utf8'));
const validate = new Ajv({ allErrors: true }).compile(schema);

function verifyArchitecture(architecture: typeof manifest, groups: Record<string, GroupConfig> = REGISTERED_GROUPS): void {
  const valid: boolean = validate(architecture);
  assert.ok(valid, JSON.stringify(validate.errors));
  const ids = architecture.components.map((component: {id: string}) => component.id);
  assert.equal(new Set(ids).size, ids.length, 'duplicate component');
  for (const component of architecture.components) {
    assert.ok(existsSync(resolve(root, component.path)), `missing component path: ${component.path}`);
    for (const dependency of component.dependsOn) assert.ok(ids.includes(dependency), `unknown component: ${dependency}`);
  }
  const layers = architecture.boundaries.map((boundary: {layer: string}) => boundary.layer);
  assert.equal(new Set(layers).size, layers.length, 'duplicate boundary');
  for (const layer of ['namespace', 'ingress', 'service_pod', 'nginx', 'schedule_gcs', 'auth_rbac', 'monitor_state', 'client_storage']) assert.ok(layers.includes(layer), `missing boundary: ${layer}`);
  for (const boundary of architecture.boundaries) assert.ok(existsSync(resolve(root, boundary.path)), `missing boundary path: ${boundary.path}`);

  const pipelineIds = architecture.pipelines.map((pipeline: {id: string}) => pipeline.id);
  assert.equal(new Set(pipelineIds).size, pipelineIds.length, 'duplicate pipeline');
  for (const pipeline of architecture.pipelines) {
    const stepIds = pipeline.steps.map((step: {id: string}) => step.id);
    assert.equal(new Set(stepIds).size, stepIds.length, `duplicate step in pipeline: ${pipeline.id}`);
    for (const step of pipeline.steps) {
      assert.ok(ids.includes(step.component), `unknown pipeline step component: ${pipeline.id}.${step.id} -> ${step.component}`);
      assert.ok(existsSync(resolve(root, step.entryPath)), `missing pipeline step entryPath: ${step.entryPath}`);
    }
  }

  const tenantIds = architecture.tenants.map((tenant: {id: string}) => tenant.id).sort();
  assert.deepEqual(tenantIds, Object.keys(registry.groups).sort());
  assert.deepEqual(tenantIds, Object.keys(groups).sort());
  assert.equal(registry.default_group, DEFAULT_GROUP_ID);
  const prefixes = new Set<string>();
  for (const tenant of architecture.tenants) {
    const group = groups[tenant.id];
    const configured = registry.groups[tenant.id];
    assert.equal(configured.id, tenant.id);
    assert.equal(group.id, tenant.id);
    assert.equal(group.name, configured.name);
    assert.equal(group.shortName, configured.short_name);
    assert.equal(group.heroImage, configured.heroImage);
    assert.deepEqual(group.theme, configured.theme);
    assert.equal(group.dataUrl, configured.data_path);
    assert.equal(group.dataUrl, tenant.dataUrl);
    assert.equal(group.fallbackDataUrl, tenant.fallbackDataUrl);
    assert.equal(group.storagePrefix, tenant.storagePrefix);
    assert.ok(!prefixes.has(group.storagePrefix), `duplicate storage prefix: ${group.storagePrefix}`);
    prefixes.add(group.storagePrefix);
    const allowedFallbacks = [group.dataUrl, ...(tenant.id === DEFAULT_GROUP_ID ? ['schedule.json'] : [])];
    assert.ok(allowedFallbacks.includes(group.fallbackDataUrl), `cross-group fallback: ${tenant.id}`);
    assert.ok(existsSync(resolve(root, configured.data_path)), `missing schedule: ${configured.data_path}`);
    assert.ok(existsSync(resolve(root, 'apps/web/public', configured.data_path)), `missing bundled schedule: ${configured.data_path}`);
    for (const host of tenant.hosts) {
      assert.equal(resolveGroupId(host.split('.')[0]), tenant.id, `host resolves to wrong group: ${host}`);
      const rule = ingress.spec.rules.find((entry: {host: string}) => entry.host === host);
      assert.ok(rule, `missing ingress host: ${host}`);
      assert.ok(rule.http.paths.some((entry: {backend: {service: {name: string; port: {name: string}}}}) => entry.backend.service.name === 'ribonclock' && entry.backend.service.port.name === 'http'), `wrong ingress backend: ${host}`);
    }
    assert.ok(tenant.hosts.includes(`${tenant.id}.iorin.net`), `missing canonical host: ${tenant.id}`);
    for (const [alias, target] of Object.entries(GROUP_ALIASES)) {
      if (target === tenant.id) assert.ok(tenant.hosts.includes(`${alias}.iorin.net`), `missing alias host: ${alias}`);
    }
  }
}

test('architecture schema and tenant configuration agree', () => verifyArchitecture(manifest));

test('generated D2 diagrams stay in sync with architecture manifest', () => {
  execFileSync(process.execPath, [resolve(root, 'scripts/gen-d2.mjs'), '--check'], { stdio: 'pipe' });
  assert.ok(existsSync(resolve(root, 'docs/architecture.svg')), 'missing rendered docs/architecture.svg');
  assert.ok(existsSync(resolve(root, 'docs/pipelines.svg')), 'missing rendered docs/pipelines.svg');
});

test('a cross-tenant fallback is rejected', () => {
  const broken = structuredClone(manifest);
  broken.tenants.find((tenant: {id: string}) => tenant.id === 'dollykiss').fallbackDataUrl = 'schedule.json';
  const brokenGroups = structuredClone(REGISTERED_GROUPS);
  brokenGroups.dollykiss.fallbackDataUrl = 'schedule.json';
  assert.throws(() => verifyArchitecture(broken, brokenGroups), /cross-group fallback/);
});

test('a pipeline referencing an unknown component is rejected', () => {
  const broken = structuredClone(manifest);
  broken.pipelines[0].steps[0].component = 'ghost-component';
  assert.throws(() => verifyArchitecture(broken), /unknown pipeline step component/);
});
