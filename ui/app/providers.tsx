'use client';

import { CookiesProvider } from 'react-cookie';

export function Providers({ children }: { children: React.ReactNode }) {
    return <CookiesProvider>{children}</CookiesProvider>;
}
