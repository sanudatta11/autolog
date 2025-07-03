export enum IncidentStatus {
  OPEN = 'OPEN',
  IN_PROGRESS = 'IN_PROGRESS',
  RESOLVED = 'RESOLVED',
  CLOSED = 'CLOSED',
  CANCELLED = 'CANCELLED'
}

export enum IncidentPriority {
  LOW = 'LOW',
  MEDIUM = 'MEDIUM',
  HIGH = 'HIGH',
  CRITICAL = 'CRITICAL'
}

export enum IncidentSeverity {
  MINOR = 'MINOR',
  MODERATE = 'MODERATE',
  MAJOR = 'MAJOR',
  CRITICAL = 'CRITICAL'
}

export enum UserRole {
  ADMIN = 'ADMIN',
  MANAGER = 'MANAGER',
  RESPONDER = 'RESPONDER',
  VIEWER = 'VIEWER'
}

export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  role: UserRole;
  avatar?: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface Incident {
  id: string;
  title: string;
  description: string;
  status: IncidentStatus;
  priority: IncidentPriority;
  severity: IncidentSeverity;
  assigneeId?: string;
  assignee?: User;
  reporterId: string;
  reporter: User;
  tags: string[];
  createdAt: Date;
  updatedAt: Date;
  resolvedAt?: Date;
  closedAt?: Date;
}

export interface IncidentUpdate {
  id: string;
  incidentId: string;
  userId: string;
  user: User;
  content: string;
  type: 'COMMENT' | 'STATUS_CHANGE' | 'ASSIGNMENT' | 'PRIORITY_CHANGE';
  metadata?: Record<string, any>;
  createdAt: Date;
}

export interface IncidentAttachment {
  id: string;
  incidentId: string;
  fileName: string;
  fileSize: number;
  mimeType: string;
  uploadedBy: string;
  uploadedAt: Date;
  url: string;
}

export interface CreateIncidentRequest {
  title: string;
  description: string;
  priority: IncidentPriority;
  severity: IncidentSeverity;
  assigneeId?: string;
  tags?: string[];
}

export interface UpdateIncidentRequest {
  title?: string;
  description?: string;
  status?: IncidentStatus;
  priority?: IncidentPriority;
  severity?: IncidentSeverity;
  assigneeId?: string;
  tags?: string[];
}

export interface IncidentFilters {
  status?: IncidentStatus[];
  priority?: IncidentPriority[];
  severity?: IncidentSeverity[];
  assigneeId?: string;
  reporterId?: string;
  tags?: string[];
  dateFrom?: Date;
  dateTo?: Date;
  search?: string;
} 