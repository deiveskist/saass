'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Loader2, Search, GitCommit, FileSearch, ClipboardList, Copy, Check } from 'lucide-react';

// Tipos espelhando o JSON que as tools Go retornam (ver mcp-tools-schema.md
// na raiz do repo).
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

type Deploy = {
  commit_sha: string;
  pr_number: number;
  title: string;
  author: string;
  merged_at: string;
  files_changed: string[];
};

type LogEntry = {
  timestamp: string;
  service: string;
  level: string;
  message: string;
  trace_id: string;
};

type InvestigationSummary = {
  ticket_id: string;
  hypothesis: string;
  confidence: string;
  evidence: string[];
  suggested_next_step: string;
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

  // Deploys
  const [repository, setRepository] = useState('');
  const [deploysSince, setDeploysSince] = useState('');
  const [deploys, setDeploys] = useState<Deploy[] | null>(null);
  const [deploysLoading, setDeploysLoading] = useState(false);
  const [deploysError, setDeploysError] = useState<string | null>(null);

  // Logs
  const [logQuery, setLogQuery] = useState('');
  const [logService, setLogService] = useState('');
  const [logStart, setLogStart] = useState('');
  const [logEnd, setLogEnd] = useState('');
  const [logs, setLogs] = useState<LogEntry[] | null>(null);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);

  // Summary
  const [hypothesis, setHypothesis] = useState('');
  const [confidence, setConfidence] = useState<'low' | 'medium' | 'high'>('medium');
  const [evidenceText, setEvidenceText] = useState('');
  const [nextStep, setNextStep] = useState('');
  const [summary, setSummary] = useState<InvestigationSummary | null>(null);
  const [summaryLoading, setSummaryLoading] = useState(false);
  const [summaryError, setSummaryError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  async function handleInvestigate(e: React.FormEvent) {
    e.preventDefault();
    if (!ticketId.trim()) return;

    setLoading(true);
    setError(null);
    setTicket(null);
    setRelated([]);
    // Limpa os resultados das outras tools ao começar uma investigação nova
    // — evita misturar dados de tickets diferentes na tela.
    setDeploys(null);
    setLogs(null);
    setSummary(null);

    try {
      const ticketResult = await callTool<Ticket>('get_ticket', { ticket_id: ticketId });
      setTicket(ticketResult);

      const relatedResult = await callTool<{ results: RelatedTicket[] }>(
        'search_related_tickets',
        { query: ticketResult.title, exclude_ticket_id: ticketId }
      );
      setRelated(relatedResult.results || []);

      // Defaults pra logs e deploys: janela de ±2h em torno da criação do
      // ticket, que é quando o problema provavelmente começou — poupa o
      // engenheiro de calcular isso na mão toda vez.
      if (ticketResult.created_at) {
        const created = new Date(ticketResult.created_at);
        setLogStart(new Date(created.getTime() - 2 * 3600_000).toISOString());
        setLogEnd(new Date(created.getTime() + 2 * 3600_000).toISOString());
        setDeploysSince(new Date(created.getTime() - 48 * 3600_000).toISOString());
      }
      setHypothesis('');
      setEvidenceText('');
      setNextStep('');
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  }

  async function handleSearchDeploys(e: React.FormEvent) {
    e.preventDefault();
    if (!repository.trim() || !deploysSince.trim()) return;

    setDeploysLoading(true);
    setDeploysError(null);
    try {
      const result = await callTool<{ deploys: Deploy[] }>('get_recent_deploys', {
        repository,
        since: deploysSince,
      });
      setDeploys(result.deploys || []);
    } catch (err) {
      setDeploysError((err as Error).message);
    } finally {
      setDeploysLoading(false);
    }
  }

  async function handleSearchLogs(e: React.FormEvent) {
    e.preventDefault();
    if (!logQuery.trim() || !logStart.trim() || !logEnd.trim()) return;

    setLogsLoading(true);
    setLogsError(null);
    try {
      const result = await callTool<{ logs: LogEntry[] }>('search_logs', {
        query: logQuery,
        start_time: logStart,
        end_time: logEnd,
        ...(logService.trim() ? { service: logService } : {}),
      });
      setLogs(result.logs || []);
    } catch (err) {
      setLogsError((err as Error).message);
    } finally {
      setLogsLoading(false);
    }
  }

  async function handleSummarize(e: React.FormEvent) {
    e.preventDefault();
    if (!ticket || !hypothesis.trim()) return;

    setSummaryLoading(true);
    setSummaryError(null);
    try {
      const evidence = evidenceText
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean);

      const result = await callTool<InvestigationSummary>('summarize_investigation', {
        ticket_id: ticket.ticket_id,
        hypothesis,
        confidence,
        evidence,
        ...(nextStep.trim() ? { suggested_next_step: nextStep } : {}),
      });
      setSummary(result);
    } catch (err) {
      setSummaryError((err as Error).message);
    } finally {
      setSummaryLoading(false);
    }
  }

  function copySummary() {
    if (!summary) return;
    const md = [
      `**Ticket:** #${summary.ticket_id}`,
      `**Hypothesis:** ${summary.hypothesis}`,
      `**Confidence:** ${summary.confidence}`,
      `**Evidence:**`,
      ...summary.evidence.map((e) => `- ${e}`),
      summary.suggested_next_step ? `**Next step:** ${summary.suggested_next_step}` : '',
    ]
      .filter(Boolean)
      .join('\n');
    navigator.clipboard.writeText(md);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
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
        <Card className="mb-6">
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

      {ticket && (
        <>
          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <GitCommit className="h-4 w-4" /> Recent deploys
              </CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSearchDeploys} className="flex flex-wrap items-end gap-4 mb-4">
                <div className="flex-1 min-w-[200px]">
                  <Label htmlFor="repository" className="mb-2">
                    Repository (owner/repo)
                  </Label>
                  <Input
                    id="repository"
                    placeholder="e.g. acme/backend"
                    value={repository}
                    onChange={(e) => setRepository(e.target.value)}
                    required
                  />
                </div>
                <div className="min-w-[220px]">
                  <Label htmlFor="deploys_since" className="mb-2">
                    Since
                  </Label>
                  <Input
                    id="deploys_since"
                    value={deploysSince}
                    onChange={(e) => setDeploysSince(e.target.value)}
                    required
                  />
                </div>
                <Button type="submit" variant="outline" disabled={deploysLoading}>
                  {deploysLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Search'}
                </Button>
              </form>
              {deploysError && <p className="text-red-500 text-sm mb-3">{deploysError}</p>}
              {deploys && deploys.length === 0 && (
                <p className="text-sm text-gray-500">No deploys found in this window.</p>
              )}
              {deploys && deploys.length > 0 && (
                <div className="space-y-3">
                  {deploys.map((d) => (
                    <div key={d.commit_sha} className="border-l-2 border-gray-200 pl-3">
                      <p className="text-sm font-medium text-gray-800">
                        {d.title}
                        {d.pr_number > 0 && (
                          <span className="text-gray-500 font-normal"> · PR #{d.pr_number}</span>
                        )}
                      </p>
                      <p className="text-xs text-gray-500">
                        {d.author} · {d.merged_at} · <code>{d.commit_sha.slice(0, 7)}</code>
                      </p>
                      {d.files_changed?.length > 0 && (
                        <p className="text-xs text-gray-400 mt-1">
                          {d.files_changed.slice(0, 5).join(', ')}
                          {d.files_changed.length > 5 ? ` +${d.files_changed.length - 5} more` : ''}
                        </p>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card className="mb-6">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileSearch className="h-4 w-4" /> Logs
              </CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSearchLogs} className="space-y-4 mb-4">
                <div className="flex flex-wrap gap-4">
                  <div className="flex-1 min-w-[200px]">
                    <Label htmlFor="log_query" className="mb-2">
                      Query
                    </Label>
                    <Input
                      id="log_query"
                      placeholder="e.g. status:error"
                      value={logQuery}
                      onChange={(e) => setLogQuery(e.target.value)}
                      required
                    />
                  </div>
                  <div className="min-w-[160px]">
                    <Label htmlFor="log_service" className="mb-2">
                      Service (optional)
                    </Label>
                    <Input
                      id="log_service"
                      placeholder="e.g. auth-service"
                      value={logService}
                      onChange={(e) => setLogService(e.target.value)}
                    />
                  </div>
                </div>
                <div className="flex flex-wrap gap-4 items-end">
                  <div className="min-w-[220px]">
                    <Label htmlFor="log_start" className="mb-2">
                      Start
                    </Label>
                    <Input
                      id="log_start"
                      value={logStart}
                      onChange={(e) => setLogStart(e.target.value)}
                      required
                    />
                  </div>
                  <div className="min-w-[220px]">
                    <Label htmlFor="log_end" className="mb-2">
                      End
                    </Label>
                    <Input
                      id="log_end"
                      value={logEnd}
                      onChange={(e) => setLogEnd(e.target.value)}
                      required
                    />
                  </div>
                  <Button type="submit" variant="outline" disabled={logsLoading}>
                    {logsLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Search'}
                  </Button>
                </div>
              </form>
              {logsError && <p className="text-red-500 text-sm mb-3">{logsError}</p>}
              {logs && logs.length === 0 && (
                <p className="text-sm text-gray-500">No log entries matched.</p>
              )}
              {logs && logs.length > 0 && (
                <div className="space-y-2 max-h-80 overflow-y-auto">
                  {logs.map((l, i) => (
                    <div key={i} className="text-xs font-mono border-b border-gray-100 pb-2">
                      <span className="text-gray-400">{l.timestamp}</span>{' '}
                      <span
                        className={
                          l.level === 'error' ? 'text-red-600 font-semibold' : 'text-gray-500'
                        }
                      >
                        [{l.level}]
                      </span>{' '}
                      <span className="text-gray-500">{l.service}</span>{' '}
                      <span className="text-gray-800">{l.message}</span>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <ClipboardList className="h-4 w-4" /> Investigation summary
              </CardTitle>
            </CardHeader>
            <CardContent>
              <form onSubmit={handleSummarize} className="space-y-4 mb-4">
                <div>
                  <Label htmlFor="hypothesis" className="mb-2">
                    Hypothesis (most likely root cause)
                  </Label>
                  <Input
                    id="hypothesis"
                    placeholder="e.g. SAML cert rotation broke SSO validation"
                    value={hypothesis}
                    onChange={(e) => setHypothesis(e.target.value)}
                    required
                  />
                </div>
                <div className="flex flex-wrap gap-4">
                  <div>
                    <Label htmlFor="confidence" className="mb-2">
                      Confidence
                    </Label>
                    <select
                      id="confidence"
                      className="border rounded-md h-9 px-3 text-sm"
                      value={confidence}
                      onChange={(e) => setConfidence(e.target.value as 'low' | 'medium' | 'high')}
                    >
                      <option value="low">Low</option>
                      <option value="medium">Medium</option>
                      <option value="high">High</option>
                    </select>
                  </div>
                  <div className="flex-1 min-w-[220px]">
                    <Label htmlFor="next_step" className="mb-2">
                      Suggested next step (optional)
                    </Label>
                    <Input
                      id="next_step"
                      value={nextStep}
                      onChange={(e) => setNextStep(e.target.value)}
                    />
                  </div>
                </div>
                <div>
                  <Label htmlFor="evidence" className="mb-2">
                    Evidence (one item per line — cite logs, related tickets, or deploys above)
                  </Label>
                  <textarea
                    id="evidence"
                    className="w-full border rounded-md p-2 text-sm min-h-[80px]"
                    value={evidenceText}
                    onChange={(e) => setEvidenceText(e.target.value)}
                  />
                </div>
                <Button
                  type="submit"
                  className="bg-orange-500 hover:bg-orange-600 text-white"
                  disabled={summaryLoading}
                >
                  {summaryLoading ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    'Generate summary'
                  )}
                </Button>
              </form>
              {summaryError && <p className="text-red-500 text-sm mb-3">{summaryError}</p>}
              {summary && (
                <div className="bg-gray-50 rounded-md p-4 relative">
                  <button
                    type="button"
                    onClick={copySummary}
                    className="absolute top-3 right-3 text-gray-400 hover:text-gray-700"
                    title="Copy as Markdown"
                  >
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </button>
                  <p className="text-sm font-medium text-gray-800 mb-1">{summary.hypothesis}</p>
                  <p className="text-xs text-gray-500 mb-3 uppercase tracking-wide">
                    Confidence: {summary.confidence}
                  </p>
                  {summary.evidence.length > 0 && (
                    <ul className="list-disc list-inside text-sm text-gray-700 space-y-1 mb-3">
                      {summary.evidence.map((e, i) => (
                        <li key={i}>{e}</li>
                      ))}
                    </ul>
                  )}
                  {summary.suggested_next_step && (
                    <p className="text-sm text-gray-700">
                      <span className="font-medium">Next step:</span>{' '}
                      {summary.suggested_next_step}
                    </p>
                  )}
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </section>
  );
}
