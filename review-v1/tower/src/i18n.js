const translations = {
  en: {
    // Nav
    inbox: 'Inbox',
    replay: 'Replay',
    learning: 'Learning',
    dashboard: 'Dashboard',

    // Inbox
    approvalInbox: 'Approval Inbox',
    pending: 'pending',
    noPendingApprovals: 'No pending approvals.',
    approve: 'Approve',
    reject: 'Reject',
    details: 'Details',
    loading: 'Loading...',
    timeLeft: 'left',
    expired: 'expired',

    // Replay
    replayTitle: 'Replay',
    selectRequest: 'Replay — Select a request',
    noRequests: 'No requests found.',
    back: 'Back',
    request: 'Request',
    requester: 'Requester',
    action: 'Action',
    target: 'Target',
    status: 'Status',
    duration: 'Duration',
    timeline: 'Timeline',
    loadingReplay: 'Loading replay...',

    // Learning
    learningProposals: 'Learning Proposals',
    analyzeNow: 'Analyze Now',
    analyzing: 'Analyzing...',
    noProposals: 'No pending proposals. Try "Analyze Now" to detect patterns.',
    accept: 'Accept',
    dismiss: 'Dismiss',
    confidence: 'confidence',
    samples: 'samples',

    // Policies
    policies: 'Policies',
    policiesTitle: 'Policies',
    newPolicy: 'New Policy',
    editPolicy: 'Edit Policy',
    noPolicies: 'No policies yet. Create one to get started.',
    policyName: 'Name',
    policyDescription: 'Description',
    policyExpression: 'CEL Expression',
    policyEffect: 'Effect',
    policyPriority: 'Priority',
    policyEnabled: 'Enabled',
    policyOrigin: 'Origin',
    policyActionType: 'Action Type',
    policyTargetSystem: 'Target System',
    policyRiskOverride: 'Risk Override',
    effectAllow: 'Allow',
    effectDeny: 'Deny',
    effectRequireApproval: 'Require Approval',
    save: 'Save',
    cancel: 'Cancel',
    archive: 'Archive',
    restore: 'Restore',
    delete: 'Delete',
    edit: 'Edit',
    confirmDelete: 'Delete permanently? This cannot be undone.',
    showArchived: 'Show archived',
    archived: 'Archived',
    learned: 'Learned',
    manual: 'Manual',
    enabled: 'Enabled',
    disabled: 'Disabled',
    optional: 'optional',
    none: 'None',

    // Dashboard
    dashboardTitle: 'Dashboard',
    totalRequests: 'Total Requests',
    allowed: 'Allowed',
    denied: 'Denied',
    pendingApproval: 'Pending Approval',
    approved: 'Approved',
    rejected: 'Rejected',
  },
  es: {
    // Nav
    inbox: 'Bandeja',
    replay: 'Replay',
    learning: 'Aprendizaje',
    dashboard: 'Panel',

    // Inbox
    approvalInbox: 'Bandeja de aprobaciones',
    pending: 'pendientes',
    noPendingApprovals: 'No hay aprobaciones pendientes.',
    approve: 'Aprobar',
    reject: 'Rechazar',
    details: 'Detalle',
    loading: 'Cargando...',
    timeLeft: 'restante',
    expired: 'expirado',

    // Replay
    replayTitle: 'Replay',
    selectRequest: 'Replay — Seleccionar request',
    noRequests: 'No hay requests.',
    back: 'Volver',
    request: 'Request',
    requester: 'Solicitante',
    action: 'Acción',
    target: 'Destino',
    status: 'Estado',
    duration: 'Duración',
    timeline: 'Línea de tiempo',
    loadingReplay: 'Cargando replay...',

    // Learning
    learningProposals: 'Propuestas de aprendizaje',
    analyzeNow: 'Analizar ahora',
    analyzing: 'Analizando...',
    noProposals: 'No hay propuestas pendientes. Probá "Analizar ahora" para detectar patrones.',
    accept: 'Aceptar',
    dismiss: 'Descartar',
    confidence: 'confianza',
    samples: 'muestras',

    // Policies
    policies: 'Políticas',
    policiesTitle: 'Políticas',
    newPolicy: 'Nueva política',
    editPolicy: 'Editar política',
    noPolicies: 'No hay políticas. Creá una para empezar.',
    policyName: 'Nombre',
    policyDescription: 'Descripción',
    policyExpression: 'Expresión CEL',
    policyEffect: 'Efecto',
    policyPriority: 'Prioridad',
    policyEnabled: 'Habilitada',
    policyOrigin: 'Origen',
    policyActionType: 'Tipo de acción',
    policyTargetSystem: 'Sistema destino',
    policyRiskOverride: 'Override de riesgo',
    effectAllow: 'Permitir',
    effectDeny: 'Denegar',
    effectRequireApproval: 'Requiere aprobación',
    save: 'Guardar',
    cancel: 'Cancelar',
    archive: 'Archivar',
    restore: 'Restaurar',
    delete: 'Eliminar',
    edit: 'Editar',
    confirmDelete: '¿Eliminar permanentemente? No se puede deshacer.',
    showArchived: 'Mostrar archivadas',
    archived: 'Archivada',
    learned: 'Aprendida',
    manual: 'Manual',
    enabled: 'Habilitada',
    disabled: 'Deshabilitada',
    optional: 'opcional',
    none: 'Ninguno',

    // Dashboard
    dashboardTitle: 'Panel',
    totalRequests: 'Total de requests',
    allowed: 'Permitidas',
    denied: 'Denegadas',
    pendingApproval: 'Pendientes de aprobación',
    approved: 'Aprobadas',
    rejected: 'Rechazadas',
  },
}

const STORAGE_KEY = 'nexus-review-lang'

export function getSavedLang() {
  return localStorage.getItem(STORAGE_KEY) || 'en'
}

export function saveLang(lang) {
  localStorage.setItem(STORAGE_KEY, lang)
}

export function t(lang, key) {
  return translations[lang]?.[key] || translations.en[key] || key
}
