import psycopg2

conn = psycopg2.connect(
    dbname="your_db",
    user="your_user",
    password="your_password",
    host="localhost",
    port="5432"
)


cur = conn.cursor()

cur.execute("SELECT version();")


record = cur.fetchone()
print("Postgres version:", record)

cur.close()
conn.close()
