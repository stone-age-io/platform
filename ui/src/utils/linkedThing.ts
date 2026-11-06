import { pb } from '@/utils/pb'

export interface LinkedThing {
  id: string
  name: string
  code: string
  active: boolean
}

/**
 * The Thing whose `nats_user` or `nebula_host` is this identity, if any.
 *
 * While that Thing is deactivated the server refuses to re-enable or credential
 * its identities (hooks/active_flag.go, guardLinkedIdentity); identity pages use
 * this to hide what will be refused. Advisory: a failure reads as "no Thing".
 */
export async function findLinkedThing(
  field: 'nats_user' | 'nebula_host',
  identityId: string,
): Promise<LinkedThing | null> {
  try {
    const t = await pb.collection('things').getFirstListItem(
      pb.filter(`${field} = {:id}`, { id: identityId }),
      { fields: 'id,name,code,active' },
    )
    return { id: t.id, name: t.name || '', code: t.code || '', active: t.active !== false }
  } catch {
    return null
  }
}
