import { NextRequest, NextResponse } from 'next/server';

// Lista de tools permitidas — evita que essa rota vire um proxy aberto
// pra qualquer path no servidor Go caso alguém adivinhe a URL.
const ALLOWED_TOOLS = new Set([
  'get_ticket',
  'search_related_tickets',
  'get_recent_deploys',
  'search_logs',
  'summarize_investigation',
]);

// Essa API route existe pra manter o INTERNAL_API_KEY só no servidor
// Next.js (nunca exposto ao browser) — o client chama /api/tools/xxx sem
// header nenhum, e é aqui que a gente injeta o Bearer token de verdade
// antes de repassar pro servidor Go (tools.ServeHTTP em httpserver.go).
export async function POST(
  request: NextRequest,
  { params }: { params: Promise<{ tool: string }> }
) {
  const { tool } = await params;

  if (!ALLOWED_TOOLS.has(tool)) {
    return NextResponse.json({ error: 'unknown tool' }, { status: 404 });
  }

  const backendUrl = process.env.MCP_HTTP_URL;
  const apiKey = process.env.INTERNAL_API_KEY;
  if (!backendUrl || !apiKey) {
    return NextResponse.json(
      { error: 'MCP_HTTP_URL / INTERNAL_API_KEY not configured on the Next.js server' },
      { status: 500 }
    );
  }

  const body = await request.text();

  let upstream: Response;
  try {
    upstream = await fetch(`${backendUrl}/api/tools/${tool}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${apiKey}`,
      },
      body,
    });
  } catch (err) {
    // Backend Go fora do ar — erro de rede, não de negócio.
    return NextResponse.json(
      { error: `failed to reach MCP backend: ${(err as Error).message}` },
      { status: 502 }
    );
  }

  const text = await upstream.text();
  return new NextResponse(text, {
    status: upstream.status,
    headers: { 'Content-Type': 'application/json' },
  });
}
