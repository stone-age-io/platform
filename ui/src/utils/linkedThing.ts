// ui/src/utils/linkedThing.ts
import { pb } from '@/utils/pb'

/** The Thing that holds an identity, as much of it as a reason needs. */
export interface LinkedThing {
  id: string
  name: string
  code: string
  active: boolean
}

/**
 * The Thing whose `nats_user` or `nebula_host` is this identity, if any.
 *
 * For a device the Thing's `active` flag is the authority: while the Thing is
 * deactivated the server refuses to re-enable its identities, or to mint them a
 * credential (hooks/active_flag.go, guardLinkedIdentity). The identity pages use
 * this to stop offering what will be refused and to say where the lever is.
 *
 * Advisory, so a failure is swallowed and reads as "no Thing": the page still
 * works without it, and the server holds the line either way.
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
