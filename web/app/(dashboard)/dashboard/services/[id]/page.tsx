import { getTeamForUser, getServiceWithConfigs } from '@/lib/db/queries';
import { redirect, notFound } from 'next/navigation';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import {
  ZendeskConfigForm,
  GithubConfigForm,
  DatadogConfigForm,
  ElasticsearchConfigForm,
} from './config-forms';
import type { IntegrationConfig } from '@/lib/db/schema';

function findConfig(configs: IntegrationConfig[], type: string) {
  return configs.find((c) => c.type === type)?.config as Record<string, unknown> | undefined;
}

export default async function ServiceDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const serviceId = Number(id);
  if (!Number.isInteger(serviceId) || serviceId <= 0) {
    notFound();
  }

  const team = await getTeamForUser();
  if (!team) {
    redirect('/sign-in');
  }

  const service = await getServiceWithConfigs(serviceId, team.id);
  if (!service) {
    notFound();
  }

  const zendesk = findConfig(service.integrationConfigs, 'zendesk');
  const github = findConfig(service.integrationConfigs, 'github');
  const datadog = findConfig(service.integrationConfigs, 'datadog');
  const elasticsearch = findConfig(service.integrationConfigs, 'elasticsearch');

  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium text-gray-900 mb-1">
        {service.name}
      </h1>
      {service.description && (
        <p className="text-sm text-gray-500 mb-6">{service.description}</p>
      )}

      <div className="space-y-6 mt-6">
        <Card>
          <CardHeader>
            <CardTitle>Zendesk</CardTitle>
          </CardHeader>
          <CardContent>
            <ZendeskConfigForm
              serviceId={service.id}
              initial={
                zendesk
                  ? { subdomain: zendesk.subdomain as string, email: zendesk.email as string }
                  : undefined
              }
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>GitHub</CardTitle>
          </CardHeader>
          <CardContent>
            <GithubConfigForm serviceId={service.id} hasExisting={!!github} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Datadog</CardTitle>
          </CardHeader>
          <CardContent>
            <DatadogConfigForm serviceId={service.id} hasExisting={!!datadog} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Elasticsearch</CardTitle>
          </CardHeader>
          <CardContent>
            <ElasticsearchConfigForm
              serviceId={service.id}
              initial={
                elasticsearch
                  ? { url: elasticsearch.url as string, index: elasticsearch.index as string }
                  : undefined
              }
            />
          </CardContent>
        </Card>
      </div>
    </section>
  );
}
