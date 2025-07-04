# Initial Data Configuration

This directory contains JSON files that define the initial users and incidents that will be seeded into the database when the application starts in development mode.

## Files

### `initial-users.json`
Contains the initial user accounts that will be created in the database.

**Structure:**
```json
{
  "users": [
    {
      "email": "user@example.com",
      "password": "password123",
      "firstName": "John",
      "lastName": "Doe",
      "role": "admin"
    }
  ]
}
```

**Available Roles:**
- `admin` - Full access to all features
- `manager` - Can manage incidents and users
- `responder` - Can view and update assigned incidents
- `viewer` - Read-only access

### `initial-incidents.json`
Contains sample incidents that will be created in the database.

**Structure:**
```json
{
  "incidents": [
    {
      "title": "Incident Title",
      "description": "Incident description",
      "status": "open",
      "priority": "high",
      "severity": "major",
      "reporterEmail": "reporter@example.com",
      "assigneeEmail": "assignee@example.com",
      "tags": ["tag1", "tag2"]
    }
  ]
}
```

**Available Statuses:**
- `open` - Incident is open and needs attention
- `in_progress` - Incident is being worked on
- `resolved` - Incident has been resolved
- `closed` - Incident is closed

**Available Priorities:**
- `low` - Low priority
- `medium` - Medium priority
- `high` - High priority
- `critical` - Critical priority

**Available Severities:**
- `minor` - Minor impact
- `moderate` - Moderate impact
- `major` - Major impact
- `critical` - Critical impact

## How It Works

1. When the backend server starts in development mode (`ENV=development`), it automatically reads these JSON files
2. Users are created with hashed passwords
3. Incidents are created and linked to existing users by email
4. If users/incidents already exist, they are skipped (no duplicates)

## Customization

To customize the initial data:

1. Edit the JSON files in this directory
2. Restart the development server
3. The new data will be seeded automatically

**Note:** Make sure that any `reporterEmail` and `assigneeEmail` values in incidents correspond to existing user emails in the users file.

## Default Login Credentials

After seeding, you can log in with any of the users defined in `initial-users.json`. The default users are:

- **Admin**: `admin@autolog.com` / `admin123`
- **Manager**: `manager@autolog.com` / `manager123`
- **Responder**: `responder@autolog.com` / `responder123`
- **Viewer**: `viewer@autolog.com` / `viewer123` 