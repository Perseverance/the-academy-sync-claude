import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

// Import translations statically to avoid dynamic import issues in build
import bgTranslations from '../public/locales/bg/translation.json';
import enTranslations from '../public/locales/en/translation.json';

const resources = {
  bg: {
    translation: bgTranslations
  },
  en: {
    translation: enTranslations
  }
};

// Only initialize i18n on the client side
if (typeof window !== 'undefined') {
  i18n
    .use(initReactI18next)
    .init({
      resources,
      lng: 'bg', // Default language
      fallbackLng: 'bg', // Fallback language
      interpolation: {
        escapeValue: false // React already safeguards from XSS
      },
      react: {
        useSuspense: false // Disable suspense to avoid issues with SSR
      },
    });
}

export default i18n;