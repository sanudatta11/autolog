# Initial Data Configuration

This directory contains JSON files that define the initial users that will be seeded into the database when the application starts in development mode.

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
- `manager` - Can manage log analysis and users
- `responder` - Can view and analyze logs
- `viewer` - Read-only access



## How It Works

1. When the backend server starts in development mode (`ENV=development`), it automatically reads these JSON files
2. Users are created with hashed passwords
3. If users already exist, they are skipped (no duplicates)

## Customization

To customize the initial data:

1. Edit the JSON files in this directory
2. Restart the development server
3. The new data will be seeded automatically



## Default Login Credentials

After seeding, you can log in with any of the users defined in `initial-users.json`. The default users are:

- **Admin**: `admin@autolog.com` / `admin123`
- **Manager**: `manager@autolog.com` / `manager123`
- **Responder**: `responder@autolog.com` / `responder123`
- **Viewer**: `viewer@autolog.com` / `viewer123` 