import Link from 'next/link';
import { getTeamForUser, getServicesForTeam } from '@/lib/db/queries';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { redirect } from 'next/navigation';
import { NewServiceForm } from './new-service-form';

export default async function ServicesPage() {
  const team = await getTeamForUser();
  if (!team) {
    redirect('/sign-in');
  }

  const services = await getServicesForTeam(team.id);

  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium text-gray-900 mb-6">
        Services
      </h1>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle>New service</CardTitle>
        </CardHeader>
        <CardContent>
          <NewServiceForm />
        </CardContent>
      </Card>

      {services.length === 0 ? (
        <p className="text-sm text-gray-500">
          No services yet — create one above to start configuring
          integrations (Zendesk, GitHub, Datadog, Elasticsearch) for it.
        </p>
      ) : (
        <div className="space-y-3">
          {services.map((service) => (
            <Link key={service.id} href={`/dashboard/services/${service.id}`}>
              <Card className="hover:border-orange-300 transition-colors">
                <CardContent className="pt-6">
                  <p className="font-medium text-gray-900">{service.name}</p>
                  {service.description && (
                    <p className="text-sm text-gray-500 mt-1">
                      {service.description}
                    </p>
                  )}
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}
