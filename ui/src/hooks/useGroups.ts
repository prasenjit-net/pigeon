import { useCallback, useState } from 'react'
import type { WsMessage } from './useWebSocket'

export interface GroupMember {
  id: string
  handle: string
  name: string
  encryptionKey: JsonWebKey
  online: boolean
}

export interface KnownGroup {
  id: string
  handle: string
  ownerId: string
  name: string
  members: GroupMember[]
}

export interface PendingGroupInvite {
  memberId: string
  group: { id: string; handle: string; name: string; ownerId: string }
  inviter: { id: string; handle: string; name: string }
}

interface GroupRosterEntry {
  group: { id: string; handle: string; name: string; ownerId: string }
  members: GroupMember[]
}

export function useGroups(ownId: string, send: (payload: object) => void) {
  const [groupsMap, setGroupsMap] = useState<Map<string, KnownGroup>>(new Map())
  const [pendingInvites, setPendingInvites] = useState<PendingGroupInvite[]>([])

  function createGroup(handle: string, name: string) {
    send({ type: 'group_create', handle, name })
  }

  function inviteToGroup(groupId: string, targetId: string) {
    send({ type: 'group_invite', groupId, targetId })
  }

  function respondToInvite(memberId: string, accept: boolean) {
    send({ type: 'group_respond', memberId, accept })
    setPendingInvites((prev) => prev.filter((i) => i.memberId !== memberId))
  }

  function leaveGroup(groupId: string) {
    send({ type: 'group_leave', groupId })
  }

  const handleMessage = useCallback(
    (msg: WsMessage) => {
      if (msg.type === 'group_roster') {
        const entries = (msg.groups as GroupRosterEntry[]) ?? []
        const next = new Map<string, KnownGroup>()
        for (const e of entries) {
          next.set(e.group.id, { ...e.group, members: e.members })
        }
        setGroupsMap(next)
      } else if (msg.type === 'group_create_ack') {
        const g = msg.group as { id: string; handle: string; name: string; ownerId: string }
        // Creator starts as the only member; they'll get a full roster after others join.
        const ownMember: GroupMember = {
          id: ownId,
          handle: '',
          name: '',
          encryptionKey: {} as JsonWebKey,
          online: true,
        }
        setGroupsMap((prev) => new Map(prev).set(g.id, { ...g, members: [ownMember] }))
      } else if (msg.type === 'pending_group_connects') {
        const invites = (msg.invites as PendingGroupInvite[]) ?? []
        setPendingInvites(invites)
      } else if (msg.type === 'group_invite') {
        const inv = msg as unknown as {
          memberId: string
          group: { id: string; handle: string; name: string; ownerId: string }
          inviter: { id: string; handle: string; name: string }
        }
        setPendingInvites((prev) => [
          ...prev.filter((i) => i.memberId !== inv.memberId),
          { memberId: inv.memberId, group: inv.group, inviter: inv.inviter },
        ])
      } else if (msg.type === 'group_user_joined') {
        const groupId = msg.groupId as string
        const member = msg.member as GroupMember
        setGroupsMap((prev) => {
          const g = prev.get(groupId)
          if (!g) return prev
          const members = g.members.some((m) => m.id === member.id)
            ? g.members.map((m) => (m.id === member.id ? { ...m, online: true } : m))
            : [...g.members, { ...member, online: true }]
          return new Map(prev).set(groupId, { ...g, members })
        })
      } else if (msg.type === 'group_user_left') {
        const groupId = msg.groupId as string
        const userId = msg.userId as string
        setGroupsMap((prev) => {
          const g = prev.get(groupId)
          if (!g) return prev
          const members = g.members.map((m) => (m.id === userId ? { ...m, online: false } : m))
          return new Map(prev).set(groupId, { ...g, members })
        })
      } else if (msg.type === 'group_member_left') {
        // A member permanently left — remove them from the group's member list.
        const groupId = msg.groupId as string
        const userId = msg.userId as string
        setGroupsMap((prev) => {
          const g = prev.get(groupId)
          if (!g) return prev
          return new Map(prev).set(groupId, { ...g, members: g.members.filter((m) => m.id !== userId) })
        })
      } else if (msg.type === 'group_leave_ack') {
        // We successfully left — remove the group from our map.
        const groupId = msg.groupId as string
        setGroupsMap((prev) => {
          const next = new Map(prev)
          next.delete(groupId)
          return next
        })
      }
      // group_respond: roster is refreshed server-side via group_roster, no extra state needed.
    },
    [ownId],
  )

  const groups = Array.from(groupsMap.values()).sort((a, b) => a.name.localeCompare(b.name))

  return {
    groups,
    groupsById: groupsMap,
    pendingInvites,
    createGroup,
    inviteToGroup,
    respondToInvite,
    leaveGroup,
    handleMessage,
  }
}
