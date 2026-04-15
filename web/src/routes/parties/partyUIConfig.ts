export interface PartyUIConfig {
  kind: 'group' | 'project'
  urlPrefix: 'groups' | 'projects'
  detailPath: (id: string) => string
  labels: { single: string; plural: string }
  writePermission: string
  memberRoleOptions: string[]
  defaultMemberRole: string
  showMemberRoleColumn: boolean
  showEntriesPanel: boolean
  cycleErrorMessage: string
}

export const groupUIConfig: PartyUIConfig = {
  kind: 'group',
  urlPrefix: 'groups',
  detailPath: (id) => `/settings/groups/${id}`,
  labels: { single: 'Group', plural: 'Groups' },
  writePermission: 'users:write',
  memberRoleOptions: ['member'],
  defaultMemberRole: 'member',
  showMemberRoleColumn: false,
  showEntriesPanel: false,
  cycleErrorMessage: "This member is already in the group's ancestry — adding them would create a cycle.",
}

export const projectUIConfig: PartyUIConfig = {
  kind: 'project',
  urlPrefix: 'projects',
  detailPath: (id) => `/settings/projects/${id}`,
  labels: { single: 'Project', plural: 'Projects' },
  writePermission: 'catalog:write',
  memberRoleOptions: ['project:owner', 'project:developer', 'project:viewer'],
  defaultMemberRole: 'project:viewer',
  showMemberRoleColumn: true,
  showEntriesPanel: true,
  cycleErrorMessage: "This member is already in the project's ancestry — adding them would create a cycle.",
}
