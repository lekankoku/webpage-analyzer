import { NextRequest } from 'next/server'

export async function GET(req: NextRequest) {
  const id = req.nextUrl.searchParams.get('id')
  if (!id) return new Response('Missing id', { status: 400 })

  const upstream = await fetch(
    `${process.env.BACKEND_URL}/analyze/stream?id=${id}`,
    {
      headers: { Accept: 'text/event-stream' },
      cache: 'no-store',
    }
  )

  if (!upstream.ok) {
    return new Response('Stream not found', { status: upstream.status })
  }

  return new Response(upstream.body, {
    headers: {
      'Content-Type': 'text/event-stream',
      'Cache-Control': 'no-cache',
      Connection: 'keep-alive',
    },
  })
}
