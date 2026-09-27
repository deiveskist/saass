import { desc, and, eq, isNull } from 'drizzle-orm';
import { db } from './drizzle';
import {
  activityLogs,
  teamMembers,
  teams,
  users,
  services,
  integrationConfigs,
  clientProfiles,
  type IntegrationType,
  type ServiceWithIntegrationConfigs,
} from './schema';
import { cookies } from 'next/headers';
import { verifyToken } from '@/lib/auth/session';

export async function getUser() {
  const sessionCookie = (await cookies()).get('session');
  if (!sessionCookie || !sessionCookie.value) {
    return null;
  }

  const sessionData = await verifyToken(sessionCookie.value);
  if (
    !sessionData ||
    !sessionData.user ||
    typeof sessionData.user.id !== 'number'
  ) {
    return null;
  }

  if (new Date(sessionData.expires) < new Date()) {
    return null;
  }

  const user = await db
    .select()
    .from(users)
    .where(and(eq(users.id, sessionData.user.id), isNull(users.deletedAt)))
    .limit(1);

  if (user.length === 0) {
    return null;
  }

  return user[0];
}

export async function getTeamByStripeCustomerId(customerId: string) {
  const result = await db
    .select()
    .from(teams)
    .where(eq(teams.stripeCustomerId, customerId))
    .limit(1);

  return result.length > 0 ? result[0] : null;
}

export async function updateTeamSubscription(
  teamId: number,
  subscriptionData: {
    stripeSubscriptionId: string | null;
    stripeProductId: string | null;
    planName: string | null;
    subscriptionStatus: string;
  }
) {
  await db
    .update(teams)
    .set({
      ...subscriptionData,
      updatedAt: new Date()
    })
    .where(eq(teams.id, teamId));
}

export async function getUserWithTeam(userId: number) {
  const result = await db
    .select({
      user: users,
      teamId: teamMembers.teamId
    })
    .from(users)
    .leftJoin(teamMembers, eq(users.id, teamMembers.userId))
    .where(eq(users.id, userId))
    .limit(1);

  return result[0];
}

export async function getActivityLogs() {
  const user = await getUser();
  if (!user) {
    throw new Error('User not authenticated');
  }

  return await db
    .select({
      id: activityLogs.id,
      action: activityLogs.action,
      timestamp: activityLogs.timestamp,
      ipAddress: activityLogs.ipAddress,
      userName: users.name
    })
    .from(activityLogs)
    .leftJoin(users, eq(activityLogs.userId, users.id))
    .where(eq(activityLogs.userId, user.id))
    .orderBy(desc(activityLogs.timestamp))
    .limit(10);
}

export async function getTeamForUser() {
  const user = await getUser();
  if (!user) {
    return null;
  }

  const result = await db.query.teamMembers.findFirst({
    where: eq(teamMembers.userId, user.id),
    with: {
      team: {
        with: {
          teamMembers: {
            with: {
              user: {
                columns: {
                  id: true,
                  name: true,
                  email: true
                }
              }
            }
          }
        }
      }
    }
  });

  return result?.team || null;
}

// --- Serviços e configuração por integração (por cliente/team) ---

export async function getServicesForTeam(teamId: number) {
  return db.query.services.findMany({
    where: eq(services.teamId, teamId),
    orderBy: (services, { desc }) => [desc(services.createdAt)],
  });
}

export async function getServiceWithConfigs(
  serviceId: number,
  teamId: number
): Promise<ServiceWithIntegrationConfigs | null> {
  // Sempre filtra por teamId também — impede um usuário de um team ver ou
  // editar o service de outro cliente só adivinhando o ID na URL.
  const result = await db.query.services.findFirst({
    where: and(eq(services.id, serviceId), eq(services.teamId, teamId)),
    with: { integrationConfigs: true },
  });
  return result ?? null;
}

export async function createService(
  teamId: number,
  name: string,
  description?: string
) {
  const [service] = await db
    .insert(services)
    .values({ teamId, name, description })
    .returning();
  return service;
}

export async function upsertIntegrationConfig(
  serviceId: number,
  type: IntegrationType,
  config: Record<string, unknown>
) {
  const [row] = await db
    .insert(integrationConfigs)
    .values({ serviceId, type, config })
    .onConflictDoUpdate({
      target: [integrationConfigs.serviceId, integrationConfigs.type],
      set: { config, updatedAt: new Date() },
    })
    .returning();
  return row;
}

export async function getClientProfile(teamId: number) {
  const result = await db.query.clientProfiles.findFirst({
    where: eq(clientProfiles.teamId, teamId),
  });
  return result ?? null;
}

export async function upsertClientProfile(
  teamId: number,
  data: {
    industry?: string;
    primaryContactName?: string;
    primaryContactEmail?: string;
    timezone?: string;
  }
) {
  const [row] = await db
    .insert(clientProfiles)
    .values({ teamId, ...data })
    .onConflictDoUpdate({
      target: clientProfiles.teamId,
      set: { ...data, updatedAt: new Date() },
    })
    .returning();
  return row;
}
