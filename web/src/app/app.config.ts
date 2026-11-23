import { ApplicationConfig, provideBrowserGlobalErrorListeners, provideZoneChangeDetection } from '@angular/core';
import { provideAnimations } from '@angular/platform-browser/animations';
import { provideHttpClient } from '@angular/common/http';
import { provideRouter } from '@angular/router';

export const appConfig: ApplicationConfig = {
  providers: [
    provideBrowserGlobalErrorListeners(),
    // PO-UI ainda depende do Zone.js para disparar a change detection
    provideZoneChangeDetection(),
    provideAnimations(),
    provideHttpClient(),
    // po-navbar, po-slide e po-link com URL interna usam routerLink, que
    // exige o Router (sem ele: NG0201 e a página inteira não renderiza).
    // A SPA não tem rotas próprias: um coringa vazio aceita / e /app/<nome>.
    provideRouter([{ path: '**', children: [] }]),
  ]
};
