import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';
import es from './locales/es.json';
import en from './locales/en.json';
import de from './locales/de.json';
import pt from './locales/pt.json';
import it from './locales/it.json';
import ru from './locales/ru.json';
import ar from './locales/ar.json';
import zh from './locales/zh.json';
import ja from './locales/ja.json';
import qu from './locales/qu.json';
import gn from './locales/gn.json';
import id from './locales/id.json';
import fr from './locales/fr.json';

i18n
    .use(LanguageDetector)
    .use(initReactI18next)
    .init({
        resources: {
            es: { translation: es },
            en: { translation: en },
            de: { translation: de },
            pt: { translation: pt },
            it: { translation: it },
            ru: { translation: ru },
            ar: { translation: ar },
            zh: { translation: zh },
            ja: { translation: ja },
            qu: { translation: qu },
            gn: { translation: gn },
            id: { translation: id },
            fr: { translation: fr },
        },
        fallbackLng: 'es',
        interpolation: {
            escapeValue: false // react already safes from xss
        },
        react: {
            useSuspense: false
        }
    });

export default i18n;
