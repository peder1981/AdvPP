import { ApplicationConfig, provideBrowserGlobalErrorListeners, provideZoneChangeDetection } from '@angular/core';
import { provideAnimations } from '@angular/platform-browser/animations';
import { provideHttpClient } from '@angular/common/http';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    // PO-UI ainda depende do Zone.js para disparar a change detection
    provideZoneChangeDetection(),
    provideAnimations(),
    provideHttpClient(),
  ]
};
