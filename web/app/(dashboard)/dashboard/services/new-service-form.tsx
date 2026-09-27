'use client';

import { useActionState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Loader2 } from 'lucide-react';
import { createServiceAction, type ServiceActionState } from './actions';

export function NewServiceForm() {
  const [state, formAction, pending] = useActionState<ServiceActionState, FormData>(
    createServiceAction,
    {}
  );

  return (
    <form action={formAction} className="flex items-end gap-4 flex-wrap">
      <div className="flex-1 min-w-[200px]">
        <Label htmlFor="name" className="mb-2">
          Name
        </Label>
        <Input id="name" name="name" placeholder="e.g. Billing API" required />
      </div>
      <div className="flex-1 min-w-[240px]">
        <Label htmlFor="description" className="mb-2">
          Description (optional)
        </Label>
        <Input id="description" name="description" placeholder="What this service does" />
      </div>
      <Button type="submit" className="bg-orange-500 hover:bg-orange-600 text-white" disabled={pending}>
        {pending ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Create'}
      </Button>
      {state.error && <p className="text-red-500 text-sm w-full">{state.error}</p>}
    </form>
  );
}
