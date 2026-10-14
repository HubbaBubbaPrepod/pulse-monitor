import { Suspense } from 'react';
import { Modules } from '@/components/modules';
export default function Page() {
  return (
    <Suspense fallback={null}>
      <Modules />
    </Suspense>
  );
}
