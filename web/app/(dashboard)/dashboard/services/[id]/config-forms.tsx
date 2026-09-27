'use client';

import { useActionState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Loader2, Check } from 'lucide-react';
import type { ServiceActionState } from '../actions';
import {
  upsertZendeskConfigAction,
  upsertGithubConfigAction,
  upsertDatadogConfigAction,
  upsertElasticsearchConfigAction,
} from '../actions';

// Cada form aqui espelha exatamente um schema de tools/logs.go, ticket.go
// etc. no lado Go — o objetivo é que o valor salvo em integration_configs
// possa ser lido direto como as env vars que o servidor Go espera hoje
// (ZENDESK_SUBDOMAIN/EMAIL/API_TOKEN, GITHUB_TOKEN, DD_API_KEY/APP_KEY,
// ELASTICSEARCH_URL/INDEX/API_KEY). Isso ainda não está fiado no servidor
// Go (que hoje só lê do ambiente do processo) — ver README para o próximo
// passo dessa integração.

function FormFeedback({ state }: { state: ServiceActionState }) {
  if (state.error) return <p className="text-red-500 text-sm mt-2">{state.error}</p>;
  if (state.success)
    return (
      <p className="text-green-600 text-sm mt-2 flex items-center gap-1">
        <Check className="h-3.5 w-3.5" /> {state.success}
      </p>
    );
  return null;
}

export function ZendeskConfigForm({
  serviceId,
  initial,
}: {
  serviceId: number;
  initial?: { subdomain?: string; email?: string };
}) {
  const [state, formAction, pending] = useActionState<ServiceActionState, FormData>(
    upsertZendeskConfigAction,
    {}
  );
  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="serviceId" value={serviceId} />
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[160px]">
          <Label htmlFor="zendesk-subdomain" className="mb-2">Subdomain</Label>
          <Input id="zendesk-subdomain" name="subdomain" placeholder="suaempresa" defaultValue={initial?.subdomain} required />
        </div>
        <div className="flex-1 min-w-[200px]">
          <Label htmlFor="zendesk-email" className="mb-2">Email</Label>
          <Input id="zendesk-email" name="email" type="email" placeholder="voce@empresa.com" defaultValue={initial?.email} required />
        </div>
        <div className="flex-1 min-w-[200px]">
          <Label htmlFor="zendesk-token" className="mb-2">API token</Label>
          <Input id="zendesk-token" name="apiToken" type="password" placeholder={initial ? '••••••••' : ''} required={!initial} />
        </div>
      </div>
      <Button type="submit" variant="outline" disabled={pending}>
        {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save Zendesk config'}
      </Button>
      <FormFeedback state={state} />
    </form>
  );
}

export function GithubConfigForm({ serviceId, hasExisting }: { serviceId: number; hasExisting?: boolean }) {
  const [state, formAction, pending] = useActionState<ServiceActionState, FormData>(
    upsertGithubConfigAction,
    {}
  );
  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="serviceId" value={serviceId} />
      <div>
        <Label htmlFor="github-token" className="mb-2">Personal access token</Label>
        <Input
          id="github-token"
          name="token"
          type="password"
          placeholder={hasExisting ? '•••••••• (deixe em branco pra manter)' : 'ghp_...'}
          required={!hasExisting}
        />
      </div>
      <Button type="submit" variant="outline" disabled={pending}>
        {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save GitHub config'}
      </Button>
      <FormFeedback state={state} />
    </form>
  );
}

export function DatadogConfigForm({ serviceId, hasExisting }: { serviceId: number; hasExisting?: boolean }) {
  const [state, formAction, pending] = useActionState<ServiceActionState, FormData>(
    upsertDatadogConfigAction,
    {}
  );
  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="serviceId" value={serviceId} />
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[200px]">
          <Label htmlFor="dd-api-key" className="mb-2">API key</Label>
          <Input
            id="dd-api-key"
            name="apiKey"
            type="password"
            placeholder={hasExisting ? '•••••••• (deixe em branco pra manter)' : ''}
            required={!hasExisting}
          />
        </div>
        <div className="flex-1 min-w-[200px]">
          <Label htmlFor="dd-app-key" className="mb-2">Application key</Label>
          <Input
            id="dd-app-key"
            name="appKey"
            type="password"
            placeholder={hasExisting ? '•••••••• (deixe em branco pra manter)' : ''}
            required={!hasExisting}
          />
        </div>
      </div>
      <Button type="submit" variant="outline" disabled={pending}>
        {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save Datadog config'}
      </Button>
      <FormFeedback state={state} />
    </form>
  );
}

export function ElasticsearchConfigForm({
  serviceId,
  initial,
}: {
  serviceId: number;
  initial?: { url?: string; index?: string };
}) {
  const [state, formAction, pending] = useActionState<ServiceActionState, FormData>(
    upsertElasticsearchConfigAction,
    {}
  );
  return (
    <form action={formAction} className="space-y-4">
      <input type="hidden" name="serviceId" value={serviceId} />
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[220px]">
          <Label htmlFor="es-url" className="mb-2">URL</Label>
          <Input id="es-url" name="url" placeholder="https://es.suaempresa.com:9200" defaultValue={initial?.url} required />
        </div>
        <div className="min-w-[160px]">
          <Label htmlFor="es-index" className="mb-2">Index</Label>
          <Input id="es-index" name="index" placeholder="logs-*" defaultValue={initial?.index || 'logs-*'} />
        </div>
      </div>
      <p className="text-xs text-gray-500">Informe API key OU usuário+senha:</p>
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[200px]">
          <Label htmlFor="es-api-key" className="mb-2">API key</Label>
          <Input id="es-api-key" name="apiKey" type="password" />
        </div>
        <div className="flex-1 min-w-[160px]">
          <Label htmlFor="es-username" className="mb-2">Username</Label>
          <Input id="es-username" name="username" />
        </div>
        <div className="flex-1 min-w-[160px]">
          <Label htmlFor="es-password" className="mb-2">Password</Label>
          <Input id="es-password" name="password" type="password" />
        </div>
      </div>
      <Button type="submit" variant="outline" disabled={pending}>
        {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Save Elasticsearch config'}
      </Button>
      <FormFeedback state={state} />
    </form>
  );
}
