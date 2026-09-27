import {
  pgTable,
  serial,
  varchar,
  text,
  timestamp,
  integer,
  jsonb,
  pgEnum,
  unique,
} from 'drizzle-orm/pg-core';
import { relations } from 'drizzle-orm';

export const users = pgTable('users', {
  id: serial('id').primaryKey(),
  name: varchar('name', { length: 100 }),
  email: varchar('email', { length: 255 }).notNull().unique(),
  passwordHash: text('password_hash').notNull(),
  role: varchar('role', { length: 20 }).notNull().default('member'),
  createdAt: timestamp('created_at').notNull().defaultNow(),
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
  deletedAt: timestamp('deleted_at'),
});

export const teams = pgTable('teams', {
  id: serial('id').primaryKey(),
  name: varchar('name', { length: 100 }).notNull(),
  createdAt: timestamp('created_at').notNull().defaultNow(),
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
  stripeCustomerId: text('stripe_customer_id').unique(),
  stripeSubscriptionId: text('stripe_subscription_id').unique(),
  stripeProductId: text('stripe_product_id'),
  planName: varchar('plan_name', { length: 50 }),
  subscriptionStatus: varchar('subscription_status', { length: 20 }),
});

export const teamMembers = pgTable('team_members', {
  id: serial('id').primaryKey(),
  userId: integer('user_id')
    .notNull()
    .references(() => users.id),
  teamId: integer('team_id')
    .notNull()
    .references(() => teams.id),
  role: varchar('role', { length: 50 }).notNull(),
  joinedAt: timestamp('joined_at').notNull().defaultNow(),
});

export const activityLogs = pgTable('activity_logs', {
  id: serial('id').primaryKey(),
  teamId: integer('team_id')
    .notNull()
    .references(() => teams.id),
  userId: integer('user_id').references(() => users.id),
  action: text('action').notNull(),
  timestamp: timestamp('timestamp').notNull().defaultNow(),
  ipAddress: varchar('ip_address', { length: 45 }),
});

export const invitations = pgTable('invitations', {
  id: serial('id').primaryKey(),
  teamId: integer('team_id')
    .notNull()
    .references(() => teams.id),
  email: varchar('email', { length: 255 }).notNull(),
  role: varchar('role', { length: 50 }).notNull(),
  invitedBy: integer('invited_by')
    .notNull()
    .references(() => users.id),
  invitedAt: timestamp('invited_at').notNull().defaultNow(),
  status: varchar('status', { length: 20 }).notNull().default('pending'),
});

// --- Cliente (perfil), serviços e configuração por integração ---
//
// "Cliente" aqui é o `team` que já existe (multi-tenancy do boilerplate) —
// clientProfiles só estende ele com dados de perfil que não fazem parte
// de billing/auth. Cada team pode ter vários `services` (um por
// produto/sistema monitorado), e cada service pode ter uma config por
// tipo de integração (Zendesk, GitHub, Datadog, Elasticsearch).

export const clientProfiles = pgTable('client_profiles', {
  id: serial('id').primaryKey(),
  teamId: integer('team_id')
    .notNull()
    .unique() // 1:1 com team — um perfil por cliente
    .references(() => teams.id),
  industry: varchar('industry', { length: 100 }),
  primaryContactName: varchar('primary_contact_name', { length: 100 }),
  primaryContactEmail: varchar('primary_contact_email', { length: 255 }),
  timezone: varchar('timezone', { length: 50 }),
  createdAt: timestamp('created_at').notNull().defaultNow(),
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
});

export const services = pgTable('services', {
  id: serial('id').primaryKey(),
  teamId: integer('team_id')
    .notNull()
    .references(() => teams.id),
  name: varchar('name', { length: 100 }).notNull(),
  description: text('description'),
  createdAt: timestamp('created_at').notNull().defaultNow(),
  updatedAt: timestamp('updated_at').notNull().defaultNow(),
});

export const integrationTypeEnum = pgEnum('integration_type', [
  'zendesk',
  'github',
  'datadog',
  'elasticsearch',
]);

export const integrationConfigs = pgTable(
  'integration_configs',
  {
    id: serial('id').primaryKey(),
    serviceId: integer('service_id')
      .notNull()
      .references(() => services.id),
    type: integrationTypeEnum('type').notNull(),
    // Cada tipo de integração tem campos bem diferentes (Zendesk precisa
    // subdomain+email+token; GitHub só token; Datadog api+app key;
    // Elasticsearch url+index+auth) — JSONB evita uma tabela por tipo ou
    // uma tabela cheia de colunas nullable. O formato de cada `type` é
    // validado no lado da aplicação (ver actions.ts), não no banco.
    //
    // NÃO GUARDE ISSO EM TEXTO PLANO EM PRODUÇÃO — são credenciais reais
    // (API tokens). Este MVP ainda não criptografa a coluna; ver o aviso
    // no README antes de rodar contra clientes reais.
    config: jsonb('config').notNull(),
    createdAt: timestamp('created_at').notNull().defaultNow(),
    updatedAt: timestamp('updated_at').notNull().defaultNow(),
  },
  (table) => ({
    // Um service só tem uma config por tipo de integração — salvar de
    // novo faz upsert, não duplica linha.
    oneConfigPerType: unique().on(table.serviceId, table.type),
  })
);

export const teamsRelations = relations(teams, ({ many, one }) => ({
  teamMembers: many(teamMembers),
  activityLogs: many(activityLogs),
  invitations: many(invitations),
  services: many(services),
  clientProfile: one(clientProfiles, {
    fields: [teams.id],
    references: [clientProfiles.teamId],
  }),
}));

export const clientProfilesRelations = relations(clientProfiles, ({ one }) => ({
  team: one(teams, {
    fields: [clientProfiles.teamId],
    references: [teams.id],
  }),
}));

export const servicesRelations = relations(services, ({ one, many }) => ({
  team: one(teams, {
    fields: [services.teamId],
    references: [teams.id],
  }),
  integrationConfigs: many(integrationConfigs),
}));

export const integrationConfigsRelations = relations(integrationConfigs, ({ one }) => ({
  service: one(services, {
    fields: [integrationConfigs.serviceId],
    references: [services.id],
  }),
}));

export const usersRelations = relations(users, ({ many }) => ({
  teamMembers: many(teamMembers),
  invitationsSent: many(invitations),
}));

export const invitationsRelations = relations(invitations, ({ one }) => ({
  team: one(teams, {
    fields: [invitations.teamId],
    references: [teams.id],
  }),
  invitedBy: one(users, {
    fields: [invitations.invitedBy],
    references: [users.id],
  }),
}));

export const teamMembersRelations = relations(teamMembers, ({ one }) => ({
  user: one(users, {
    fields: [teamMembers.userId],
    references: [users.id],
  }),
  team: one(teams, {
    fields: [teamMembers.teamId],
    references: [teams.id],
  }),
}));

export const activityLogsRelations = relations(activityLogs, ({ one }) => ({
  team: one(teams, {
    fields: [activityLogs.teamId],
    references: [teams.id],
  }),
  user: one(users, {
    fields: [activityLogs.userId],
    references: [users.id],
  }),
}));

export type User = typeof users.$inferSelect;
export type NewUser = typeof users.$inferInsert;
export type Team = typeof teams.$inferSelect;
export type NewTeam = typeof teams.$inferInsert;
export type TeamMember = typeof teamMembers.$inferSelect;
export type NewTeamMember = typeof teamMembers.$inferInsert;
export type ActivityLog = typeof activityLogs.$inferSelect;
export type NewActivityLog = typeof activityLogs.$inferInsert;
export type Invitation = typeof invitations.$inferSelect;
export type NewInvitation = typeof invitations.$inferInsert;
export type ClientProfile = typeof clientProfiles.$inferSelect;
export type NewClientProfile = typeof clientProfiles.$inferInsert;
export type Service = typeof services.$inferSelect;
export type NewService = typeof services.$inferInsert;
export type IntegrationConfig = typeof integrationConfigs.$inferSelect;
export type NewIntegrationConfig = typeof integrationConfigs.$inferInsert;
export type IntegrationType = (typeof integrationTypeEnum.enumValues)[number];
export type ServiceWithIntegrationConfigs = Service & {
  integrationConfigs: IntegrationConfig[];
};
export type TeamDataWithMembers = Team & {
  teamMembers: (TeamMember & {
    user: Pick<User, 'id' | 'name' | 'email'>;
  })[];
};

export enum ActivityType {
  SIGN_UP = 'SIGN_UP',
  SIGN_IN = 'SIGN_IN',
  SIGN_OUT = 'SIGN_OUT',
  UPDATE_PASSWORD = 'UPDATE_PASSWORD',
  DELETE_ACCOUNT = 'DELETE_ACCOUNT',
  UPDATE_ACCOUNT = 'UPDATE_ACCOUNT',
  CREATE_TEAM = 'CREATE_TEAM',
  REMOVE_TEAM_MEMBER = 'REMOVE_TEAM_MEMBER',
  INVITE_TEAM_MEMBER = 'INVITE_TEAM_MEMBER',
  ACCEPT_INVITATION = 'ACCEPT_INVITATION',
}
