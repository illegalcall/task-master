'use client';

import { useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { createClient } from '@supabase/supabase-js';

export default function AuthCallbackPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  
  useEffect(() => {
    const handleCallback = async () => {
      try {
        const supabaseUrl = `https://${process.env.NEXT_PUBLIC_SUPABASE_URL}`;
        const supabaseKey = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY as string;
        const supabase = createClient(supabaseUrl, supabaseKey);
        
        // Get the code from the query parameters
        const code = searchParams.get('code');
        
        if (code) {
          // Exchange the code for a session
          const { data, error } = await supabase.auth.exchangeCodeForSession(code);
          
          if (error) {
            console.error('Error exchanging code for session:', error);
            router.push('/sign-up?error=Authentication failed');
            return;
          }
          
          if (data.user) {
            console.log('User authenticated:', data.user.id);
            
            // Create profile in our backend
            try {
              const response = await fetch(`${process.env.NEXT_PUBLIC_API_URL}/api/users`, {
                method: 'POST',
                headers: {
                  'Content-Type': 'application/json',
                },
                body: JSON.stringify({ 
                  user_id: data.user.id 
                }),
              });
              
              if (!response.ok) {
                console.warn('Failed to create profile, but continuing:', await response.text());
              } else {
                console.log('Profile created successfully');
              }
            } catch (err) {
              console.warn('Error creating profile, but continuing:', err);
            }
            
            // Redirect to dashboard or home page
            router.push('/');
          }
        } else {
          console.error('No code found in redirect');
          router.push('/sign-up?error=No authentication code received');
        }
      } catch (err) {
        console.error('Unexpected error during auth callback:', err);
        router.push('/sign-up?error=Authentication process failed');
      }
    };
    
    handleCallback();
  }, [router, searchParams]);
  
  return (
    <div className="flex h-screen w-full items-center justify-center">
      <div className="text-center">
        <h2 className="text-xl font-medium mb-2">Completing authentication...</h2>
        <p className="text-muted-foreground">Please wait while we log you in.</p>
      </div>
    </div>
  );
} 