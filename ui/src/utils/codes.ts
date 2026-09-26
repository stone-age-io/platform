import { pb } from '@/utils/pb'

// Code suggestions come from the server (GET /api/codes/suggest, hooks/codes.go)
// and there is deliberately no generator here. A second implementation in
// TypeScript would drift from the Go one, and a code is frozen and printed, so
// the drift would be permanent. See ADR 0003 in platform-docs.
//
// A suggestion is not reserved. It is free when it is returned; if another save
// takes it first, the unique index refuses the create and the user asks again.
export type CodeKind = 'thing' | 'location'

export async function suggestCode(kind: CodeKind, typeId?: string): Promise<string> {
  const query: Record<string, string> = { kind, count: '1' }
  if (typeId) query.type = typeId
  const res = await pb.send<{ codes?: string[] }>('/api/codes/suggest', { method: 'GET', query })
  return res.codes?.[0] || ''
}
