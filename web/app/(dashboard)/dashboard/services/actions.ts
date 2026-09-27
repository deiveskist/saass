'use server';

import { z } from 'zod';
import { validatedActionWithUser } from '@/lib/auth/middleware';
import {
  createService,
  getServiceWithConfigs,
  getUserWithTeam,
  upsertIntegrationConfig,
} from '@/lib/db/queries';
import type { IntegrationType } from '@/lib/db/schema';
import { revalidatePath } from 'next/cache';
import { redirect } from 'next/navigation';

export type ServiceActionState = {
  error?: string;
  success?: string;
};

// Toda action aqui usa validatedActionWithUser (não withTeam) porque
// precisamos do formato { error } / { success } que useActionState espera
// pra mostrar feedback inline no formulário — withTeam é pra actions que
// só redirecionam ou lançam erro (ver lib/payments/actions.ts).
async function requireTeamId(userId: number): Promise<number> {
  const userWithTeam = await getUserWithTeam(userId);
  if (!userWithTeam?.teamId) {
    throw new Error('User is not part of a team');
  }
  return userWithTeam.teamId;
}

const createServiceSchema = z.object({
  name: z.string().min(1, 'Nome é obrigatório').max(100),
  description: z.string().max(500).optional(),
});

export const createServiceAction = validatedActionWithUser(
  createServiceSchema,
  async (data, _formData, user) => {
    const teamId = await requireTeamId(user.id);
    const service = await createService(teamId, data.name, data.description);
    revalidatePath('/dashboard/services');
    redirect(`/dashboard/services/${service.id}`);
  }
);

// Cada tipo de integração tem seu próprio schema e sua própria action —
// os campos são bem diferentes entre eles (ver comentário em schema.ts),
// então validar cada um separadamente é mais claro do que um schema
// genérico com tudo opcional.
const serviceIdSchema = z.coerce.number().int().positive();

async function getServiceAndExistingConfig(
  serviceId: number,
  teamId: number,
  type: IntegrationType
): Promise<Record<string, unknown> | undefined> {
  const service = await getServiceWithConfigs(serviceId, teamId);
  if (!service) {
    throw new Error('Service not found for this team');
  }
  return service.integrationConfigs.find((c) => c.type === type)?.config as
    | Record<string, unknown>
    | undefined;
}

// mergeSecret evita sobrescrever um segredo já salvo quando o campo vem
// vazio numa atualização — o usuário só reenvia esse campo se quiser
// trocar o valor, não precisa retypar toda vez que só quer mudar outro
// campo (ex.: o índice do Elasticsearch).
function mergeSecret(newValue: string | undefined, existing: unknown): string | undefined {
  if (newValue && newValue.trim() !== '') return newValue;
  return typeof existing === 'string' ? existing : undefined;
}

const zendeskConfigSchema = z.object({
  serviceId: serviceIdSchema,
  subdomain: z.string().min(1, 'Subdomain é obrigatório'),
  email: z.string().email('Email inválido'),
  apiToken: z.string().optional(),
});

export const upsertZendeskConfigAction = validatedActionWithUser(
  zendeskConfigSchema,
  async (data, _formData, user) => {
    const teamId = await requireTeamId(user.id);
    const { serviceId, ...input } = data;
    const existing = await getServiceAndExistingConfig(serviceId, teamId, 'zendesk');

    const apiToken = mergeSecret(input.apiToken, existing?.apiToken);
    if (!apiToken) {
      return { error: 'API token é obrigatório' };
    }

    await upsertIntegrationConfig(serviceId, 'zendesk', { ...input, apiToken });
    revalidatePath(`/dashboard/services/${serviceId}`);
    return { success: 'Configuração do Zendesk salva.' };
  }
);

const githubConfigSchema = z.object({
  serviceId: serviceIdSchema,
  token: z.string().optional(),
});

export const upsertGithubConfigAction = validatedActionWithUser(
  githubConfigSchema,
  async (data, _formData, user) => {
    const teamId = await requireTeamId(user.id);
    const { serviceId, token: inputToken } = data;
    const existing = await getServiceAndExistingConfig(serviceId, teamId, 'github');

    const token = mergeSecret(inputToken, existing?.token);
    if (!token) {
      return { error: 'Token é obrigatório' };
    }

    await upsertIntegrationConfig(serviceId, 'github', { token });
    revalidatePath(`/dashboard/services/${serviceId}`);
    return { success: 'Configuração do GitHub salva.' };
  }
);

const datadogConfigSchema = z.object({
  serviceId: serviceIdSchema,
  apiKey: z.string().optional(),
  appKey: z.string().optional(),
});

export const upsertDatadogConfigAction = validatedActionWithUser(
  datadogConfigSchema,
  async (data, _formData, user) => {
    const teamId = await requireTeamId(user.id);
    const { serviceId, ...input } = data;
    const existing = await getServiceAndExistingConfig(serviceId, teamId, 'datadog');

    const apiKey = mergeSecret(input.apiKey, existing?.apiKey);
    const appKey = mergeSecret(input.appKey, existing?.appKey);
    if (!apiKey || !appKey) {
      return { error: 'API key e Application key são obrigatórias' };
    }

    await upsertIntegrationConfig(serviceId, 'datadog', { apiKey, appKey });
    revalidatePath(`/dashboard/services/${serviceId}`);
    return { success: 'Configuração do Datadog salva.' };
  }
);

const elasticsearchConfigSchema = z.object({
  serviceId: serviceIdSchema,
  url: z.string().url('URL inválida'),
  index: z.string().min(1).default('logs-*'),
  apiKey: z.string().optional(),
  username: z.string().optional(),
  password: z.string().optional(),
});

export const upsertElasticsearchConfigAction = validatedActionWithUser(
  elasticsearchConfigSchema,
  async (data, _formData, user) => {
    const teamId = await requireTeamId(user.id);
    const { serviceId, url, index, ...input } = data;
    const existing = await getServiceAndExistingConfig(serviceId, teamId, 'elasticsearch');

    const apiKey = mergeSecret(input.apiKey, existing?.apiKey);
    const username = mergeSecret(input.username, existing?.username);
    const password = mergeSecret(input.password, existing?.password);

    if (!apiKey && !(username && password)) {
      return { error: 'Informe API key OU usuário+senha' };
    }

    await upsertIntegrationConfig(serviceId, 'elasticsearch', {
      url,
      index,
      ...(apiKey ? { apiKey } : { username, password }),
    });
    revalidatePath(`/dashboard/services/${serviceId}`);
    return { success: 'Configuração do Elasticsearch salva.' };
  }
);
