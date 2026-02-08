FROM nginx:1.27-alpine

RUN rm /etc/nginx/conf.d/default.conf

COPY nginx/nginx.conf /etc/nginx/templates/nginx.conf.template
COPY frontend/ /usr/share/nginx/html/

EXPOSE 80