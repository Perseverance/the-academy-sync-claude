# Academy Sync - Web Application

React/Next.js frontend for the Academy Sync application - automated Strava to Google Sheets synchronization.

## Overview

This is the web interface for Academy Sync, providing users with authentication, configuration, and monitoring capabilities for their automated Strava running log synchronization.

### Features

- **User Authentication**: Google OAuth integration for secure access
- **Dashboard**: Overview of sync status and configuration
- **Activity Logs**: View processing history and sync results
- **Connection Management**: Configure Strava and Google Sheets integrations
- **Manual Sync**: Trigger on-demand synchronization
- **Responsive Design**: Mobile-first approach with modern UI

## Technology Stack

- **Framework**: Next.js 15 with App Router
- **Language**: TypeScript
- **Styling**: Tailwind CSS
- **UI Components**: shadcn/ui (Radix UI primitives)
- **State Management**: React Context
- **Package Manager**: pnpm (preferred) or npm

## Development Setup

### Prerequisites

- Node.js 18+ 
- pnpm (recommended) or npm
- Docker (for containerized development)

### Local Development

1. **Install dependencies:**
   ```bash
   pnpm install
   # or
   npm install
   ```

2. **Start development server:**
   ```bash
   pnpm dev
   # or
   npm run dev
   ```

3. **Open in browser:**
   ```
   http://localhost:3000
   ```

### Available Scripts

- `pnpm dev` - Start development server (hot reload enabled)
- `pnpm build` - Build for production
- `pnpm start` - Start production server
- `pnpm lint` - Run ESLint
- `pnpm type-check` - Run TypeScript compiler check

## Docker Deployment

### Build Docker Image

```bash
docker build -t academy-sync-web .
```

### Run Container

```bash
docker run -p 8080:8080 academy-sync-web
```

The application will be available at `http://localhost:8080`

### Docker Compose

For local development with the full stack:

```bash
# From project root
docker-compose up --build
```

## Project Structure

```
web/
├── app/                      # Next.js App Router pages
│   ├── auth/                 # Authentication routes
│   ├── dashboard/            # Dashboard page
│   ├── logs/                 # Activity logs page
│   ├── layout.tsx            # Root layout
│   └── page.tsx              # Home page
├── components/               # React components
│   ├── ui/                   # shadcn/ui components
│   ├── icons/                # SVG icon components
│   ├── auth-provider.tsx     # Authentication context
│   └── *.tsx                 # Feature components
├── context/                  # React contexts
├── hooks/                    # Custom React hooks
├── lib/                      # Utility functions
├── public/                   # Static assets
├── styles/                   # Global styles
├── Dockerfile                # Container configuration
├── next.config.mjs           # Next.js configuration
├── tailwind.config.ts        # Tailwind CSS configuration
├── tsconfig.json             # TypeScript configuration
└── package.json              # Dependencies and scripts
```

## Configuration

The application uses Next.js configuration for:

- **Standalone output**: Optimized for containerization
- **Image optimization**: Disabled for static deployment compatibility
- **Build settings**: TypeScript and ESLint error handling

## Authentication Flow

1. User visits the application
2. Redirected to Google OAuth if not authenticated
3. After successful authentication, redirected to dashboard
4. Authentication state managed via React Context
5. Protected routes require valid authentication

## API Integration

The web application integrates with the Academy Sync backend services:

- **Backend API**: User management and configuration
- **Automation Engine**: Sync status and manual triggers
- **Notification Service**: Email preferences and history

## Deployment

### Production Build

```bash
pnpm build
```

### Environment Variables

The application uses the following environment variables:

```bash
# Required for Strava OAuth integration
NEXT_PUBLIC_STRAVA_CLIENT_ID=your_strava_client_id

# Required for Google OAuth (public client ID only)
NEXT_PUBLIC_GOOGLE_CLIENT_ID=your_google_client_id

# API endpoints (configured automatically in development)
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Copy `.env.example` to `.env.local` and configure the required values:

```bash
cp .env.example .env.local
```

### Vercel Deployment

The application is configured for deployment on Vercel:

1. Connect your GitHub repository
2. Vercel will automatically detect Next.js
3. Configure environment variables in Vercel dashboard
4. Deploy with automatic builds on push

### Docker Production

For production deployment using Docker:

```bash
# Build production image
docker build -t academy-sync-web:latest .

# Run with production settings
docker run -d \
  --name academy-sync-web \
  -p 8080:8080 \
  academy-sync-web:latest
```

## Contributing

1. Follow the existing code structure and patterns
2. Use TypeScript for type safety
3. Follow the established component organization
4. Ensure responsive design principles
5. Test Docker builds before submitting PRs

## Support

For issues related to the web application, please create an issue in the main repository with the `Component: Frontend` label.