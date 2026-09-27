'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Loader2, Search } from 'lucide-react';

// Tipos espelhando o JSON que as tools Go retornam (ver mcp-tools-schema.md
// na raiz do repo) — mantidos aqui em vez de gerados, já que o v1 é só
// essas 2 tools na UI.
type Comment = {
  author: string;
  body: string;
  created_at: string;
};

type Ticket = {
  ticket_id: string;
  title: string;
  description: string;
  status: string;
  priority: string;
  created_at: string;
  comments: Comment[];
  tags: string[];
};

type RelatedTicket = {
  ticket_id: string;
  title: string;
  resolution_summary: string | null;
  resolved_at: string | null;
};

async function callTool<T>(tool: string, args: Record<string, unknown>): Promise<T> {
  const res = await fetch(`/api/tools/${tool}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(args),
  });
  const data = await res.json();
  if (!res.ok) {
    // Tanto erro de validação (400) quanto de integração upstream (502)
    // vêm nesse formato { error: "..." } vindo do servidor Go.
    throw new Error(data.error || `request failed with status ${res.status}`);
  }
  return data as T;
}

export default function InvestigatePage() {
  const [ticketId, setTicketId] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [ticket, setTicket] = useState<Ticket | null>(null);
  const [related, setRelated] = useState<RelatedTicket[]>([]);

  async function handleInvestigate(e: React.FormEvent) {
    e.preventDefault();
    if (!ticketId.trim()) return;

    setLoading(true);
    setError(null);
    setTicket(null);
    setRelated([]);

    try {
      const ticketResult = await callTool<Ticket>('get_ticket', { ticket_id: ticketId });
      setTicket(ticketResult);

      // Busca tickets relacionados usando o título como query — heurística
      // simples pro v1; o agente Python pode mandar uma query melhor
      // (extraída da descrição/erro) quando estiver no loop.
      const relatedResult = await callTool<{ results: RelatedTicket[] }>(
        'search_related_tickets',
        { query: ticketResult.title, exclude_ticket_id: ticketId }
      );
      setRelated(relatedResult.results || []);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="flex-1 p-4 lg:p-8">
      <h1 className="text-lg lg:text-2xl font-medium text-gray-900 mb-6">
        Investigate a ticket
      </h1>

      <Card className="mb-6">
        <CardContent className="pt-6">
          <form onSubmit={handleInvestigate} className="flex items-end gap-4">
            <div className="flex-1">
              <Label htmlFor="ticket_id" className="mb-2">
                Ticket ID
              </Label>
              <Input
                id="ticket_id"
                placeholder="e.g. 12345"
                value={ticketId}
                onChange={(e) => setTicketId(e.target.value)}
                required
              />
            </div>
            <Button
              type="submit"
              className="bg-orange-500 hover:bg-orange-600 text-white"
              disabled={loading}
            >
              {loading ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  Investigating...
                </>
              ) : (
                <>
                  <Search className="mr-2 h-4 w-4" />
                  Investigate
                </>
              )}
            </Button>
          </form>
          {error && <p className="text-red-500 text-sm mt-3">{error}</p>}
        </CardContent>
      </Card>

      {ticket && (
        <Card className="mb-6">
          <CardHeader>
            <CardTitle className="flex items-center justify-between">
              <span>{ticket.title}</span>
              <span className="text-sm font-normal text-gray-500">
                {ticket.status} · {ticket.priority}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-gray-700 mb-4">{ticket.description}</p>
            {ticket.tags?.length > 0 && (
              <div className="flex gap-2 mb-4 flex-wrap">
                {ticket.tags.map((tag) => (
                  <span
                    key={tag}
                    className="text-xs bg-gray-100 text-gray-600 px-2 py-1 rounded"
                  >
                    {tag}
                  </span>
                ))}
              </div>
            )}
            <h3 className="text-sm font-medium text-gray-900 mb-2">
              Comments ({ticket.comments?.length ?? 0})
            </h3>
            <div className="space-y-3">
              {ticket.comments?.map((c, i) => (
                <div key={i} className="border-l-2 border-gray-200 pl-3">
                  <p className="text-sm font-medium text-gray-800">{c.author}</p>
                  <p className="text-sm text-gray-600">{c.body}</p>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {related.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Related tickets</CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            {related.map((r) => (
              <div key={r.ticket_id} className="border-b border-gray-100 pb-3 last:border-0">
                <p className="text-sm font-medium text-gray-800">
                  #{r.ticket_id} — {r.title}
                </p>
                {r.resolution_summary && (
                  <p className="text-sm text-gray-600 mt-1">{r.resolution_summary}</p>
                )}
              </div>
            ))}
          </CardContent>
        </Card>
      )}
    </section>
  );
}
