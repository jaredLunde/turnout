const port = process.env.PORT || 3000;
const dbUrl = process.env.DATABASE_URL;
const apiKey = process.env.API_KEY;

if (process.env.NODE_ENV === 'production') {
  console.log('Running in production');
}